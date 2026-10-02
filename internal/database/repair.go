package database

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/blacktop/lporg/internal/utils"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"gorm.io/gorm"
)

type sqliteTrigger struct {
	Name string `gorm:"column:name"`
	SQL  string `gorm:"column:sql"`
}

type appIDRow struct {
	ID int `gorm:"column:item_id"`
}

type itemIDRow struct {
	ID int `gorm:"column:rowid"`
}

type groupIDRow struct {
	ID int `gorm:"column:item_id"`
}

type folderPlacement struct {
	FolderID     int
	ParentPageID int
	Ordering     int
	Name         string
}

type folderPagePlacement struct {
	FolderPageID int
	FolderID     int
	Ordering     int
	FolderName   string
}

type appPlacement struct {
	Title    string
	AppID    int
	ParentID int
	Ordering int
}

type defaultItem struct {
	ID       int
	UUID     string
	Type     int
	ParentID int
	Ordering int
	Flags    int
}

// RepairFromConfig Rebuilds The Launchpad Page And Folder Layout Exactly From The Loaded YAML Config.
func (lp *LaunchPad) RepairFromConfig() error {
	utils.Indent(log.Info, 2)("Repairing Launchpad Database")

	tx := lp.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("Starting Repair Transaction Failed: %w", tx.Error)
	}

	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	var triggers []sqliteTrigger
	if err := tx.Raw(`
		SELECT name, sql
		FROM sqlite_master
		WHERE type = 'trigger'
		AND sql IS NOT NULL
		ORDER BY name
	`).Scan(&triggers).Error; err != nil {
		return fmt.Errorf("Loading SQLite Triggers Failed: %w", err)
	}

	utils.Indent(log.WithField("count", len(triggers)).Info, 3)("Temporarily Dropping SQLite Triggers")
	for _, trigger := range triggers {
		if err := tx.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s"`, escapeSQLiteIdentifier(trigger.Name))).Error; err != nil {
			return fmt.Errorf("Dropping SQLite Trigger %q Failed: %w", trigger.Name, err)
		}
	}

	if err := tx.Exec("UPDATE dbinfo SET value = 1 WHERE key = 'ignore_items_update_triggers'").Error; err != nil {
		return fmt.Errorf("Disabling Launchpad Update Triggers Failed: %w", err)
	}

	utils.Indent(log.Info, 3)("Clearing Existing Launchpad Pages And Folders")
	if err := tx.Exec("DELETE FROM groups").Error; err != nil {
		return fmt.Errorf("Clearing Groups Failed: %w", err)
	}
	if err := tx.Exec("DELETE FROM items WHERE type IN (?, ?, ?)", RootType, FolderRootType, PageType).Error; err != nil {
		return fmt.Errorf("Clearing Page And Folder Items Failed: %w", err)
	}

	utils.Indent(log.Info, 3)("Rebuilding Launchpad Roots")
	defaults := []defaultItem{
		{ID: 1, UUID: "ROOTPAGE", Type: RootType, ParentID: 0, Ordering: 0, Flags: 0},
		{ID: 2, UUID: "HOLDINGPAGE", Type: PageType, ParentID: 1, Ordering: 0, Flags: 2},
		{ID: 4, UUID: "HOLDINGPAGE_DB", Type: PageType, ParentID: 3, Ordering: 0, Flags: 2},
		{ID: 5, UUID: "ROOTPAGE_VERS", Type: RootType, ParentID: 0, Ordering: 0, Flags: 0},
		{ID: 6, UUID: "HOLDINGPAGE_VERS", Type: PageType, ParentID: 5, Ordering: 0, Flags: 2},
	}

	for _, item := range defaults {
		if err := insertRepairItem(tx, item.ID, item.Type, item.ParentID, item.Ordering, item.Flags, item.UUID); err != nil {
			return err
		}
		if err := insertRepairGroup(tx, item.ID, ""); err != nil {
			return err
		}
	}

	nextID, err := getNextRepairID(tx)
	if err != nil {
		return err
	}

	placedAppIDs := make(map[int]bool)
	var createdPageIDs []int
	var folderPlacements []folderPlacement
	var folderPagePlacements []folderPagePlacement
	var appPlacements []appPlacement

	placeApp := func(title string, parentID, ordering int) error {
		appIDs, err := findRepairAppIDs(tx, title)
		if err != nil {
			return err
		}
		if len(appIDs) == 0 {
			utils.Indent(log.WithField("app", title).Warn, 4)("Skipping Missing App")
			return nil
		}

		chosenID := 0
		for _, appID := range appIDs {
			if !placedAppIDs[appID] {
				chosenID = appID
				break
			}
		}
		if chosenID == 0 {
			chosenID = appIDs[0]
		}

		if err := tx.Exec(`
			UPDATE items
			SET parent_id = ?, ordering = ?, type = ?
			WHERE rowid = ?
		`, parentID, ordering, ApplicationType, chosenID).Error; err != nil {
			return fmt.Errorf("Placing App %q Failed: %w", title, err)
		}

		placedAppIDs[chosenID] = true
		appPlacements = append(appPlacements, appPlacement{
			Title:    title,
			AppID:    chosenID,
			ParentID: parentID,
			Ordering: ordering,
		})

		for _, duplicateID := range appIDs {
			if duplicateID == chosenID {
				continue
			}
			if err := tx.Exec(`
				UPDATE items
				SET parent_id = 6, ordering = -1, type = ?
				WHERE rowid = ?
			`, ApplicationType, duplicateID).Error; err != nil {
				return fmt.Errorf("Hiding Duplicate App %q Failed: %w", title, err)
			}
		}

		return nil
	}

	utils.Indent(log.Info, 3)("Rebuilding Pages From YAML")
	contentPages := nonEmptyRepairPages(lp.Config.Apps.Pages)

	for pageIndex, page := range contentPages {
		pageID := nextID()
		pageOrder := pageIndex + 1

		if err := insertRepairItem(tx, pageID, PageType, 1, pageOrder, 2, ""); err != nil {
			return err
		}
		if err := insertRepairGroup(tx, pageID, ""); err != nil {
			return err
		}

		createdPageIDs = append(createdPageIDs, pageID)

		for itemIndex, item := range page.Items {
			switch typedItem := item.(type) {
			case string:
				if err := placeApp(typedItem, pageID, itemIndex); err != nil {
					return err
				}
			default:
				var folder AppFolder
				if err := mapstructure.Decode(item, &folder); err != nil {
					return fmt.Errorf("Decoding Folder From YAML Failed: %w", err)
				}

				folderID := nextID()
				folderFlags := 0
				if folder.Name == "Utilities" {
					folderFlags = 1
				}

				if err := insertRepairItem(tx, folderID, FolderRootType, pageID, itemIndex, folderFlags, ""); err != nil {
					return err
				}
				if err := insertRepairGroup(tx, folderID, folder.Name); err != nil {
					return err
				}

				folderPlacements = append(folderPlacements, folderPlacement{
					FolderID:     folderID,
					ParentPageID: pageID,
					Ordering:     itemIndex,
					Name:         folder.Name,
				})

				for folderPageIndex, folderPage := range folder.Pages {
					folderPageID := nextID()
					folderPageOrder := folderPage.Number
					if folderPageOrder == 0 {
						folderPageOrder = folderPageIndex + 1
					}

					if err := insertRepairItem(tx, folderPageID, PageType, folderID, folderPageOrder, 2, ""); err != nil {
						return err
					}
					if err := insertRepairGroup(tx, folderPageID, ""); err != nil {
						return err
					}

					folderPagePlacements = append(folderPagePlacements, folderPagePlacement{
						FolderPageID: folderPageID,
						FolderID:     folderID,
						Ordering:     folderPageOrder,
						FolderName:   folder.Name,
					})

					for appIndex, appTitle := range folderPage.Items {
						if err := placeApp(appTitle, folderPageID, appIndex); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	if err := hideUnplacedRepairApps(tx, placedAppIDs); err != nil {
		return err
	}

	utils.Indent(log.Info, 3)("Removing Literal Other Folder")
	if err := removeLiteralOtherRepairFolders(tx); err != nil {
		return err
	}

	utils.Indent(log.Info, 3)("Applying Exact YAML Ordering")
	for pageIndex, pageID := range createdPageIDs {
		if err := tx.Exec(`
			UPDATE items
			SET parent_id = 1, ordering = ?, type = ?
			WHERE rowid = ?
		`, pageIndex+1, PageType, pageID).Error; err != nil {
			return fmt.Errorf("applying page order failed: %w", err)
		}
	}

	for _, folder := range folderPlacements {
		if err := tx.Exec(`
			UPDATE items
			SET parent_id = ?, ordering = ?, type = ?
			WHERE rowid = ?
		`, folder.ParentPageID, folder.Ordering, FolderRootType, folder.FolderID).Error; err != nil {
			return fmt.Errorf("applying folder order for %q failed: %w", folder.Name, err)
		}
	}

	for _, folderPage := range folderPagePlacements {
		if err := tx.Exec(`
			UPDATE items
			SET parent_id = ?, ordering = ?, type = ?
			WHERE rowid = ?
		`, folderPage.FolderID, folderPage.Ordering, PageType, folderPage.FolderPageID).Error; err != nil {
			return fmt.Errorf("applying folder page order for %q failed: %w", folderPage.FolderName, err)
		}
	}

	for _, app := range appPlacements {
		if err := tx.Exec(`
			UPDATE items
			SET parent_id = ?, ordering = ?, type = ?
			WHERE rowid = ?
		`, app.ParentID, app.Ordering, ApplicationType, app.AppID).Error; err != nil {
			return fmt.Errorf("applying app order for %q failed: %w", app.Title, err)
		}
	}

	for _, app := range appPlacements {
		appIDs, err := findRepairAppIDs(tx, app.Title)
		if err != nil {
			return err
		}
		for _, duplicateID := range appIDs {
			if duplicateID == app.AppID {
				continue
			}
			if err := tx.Exec(`
				UPDATE items
				SET parent_id = 6, ordering = -1, type = ?
				WHERE rowid = ?
			`, ApplicationType, duplicateID).Error; err != nil {
				return fmt.Errorf("hiding duplicate app %q failed: %w", app.Title, err)
			}
		}
	}

	if err := tx.Exec("UPDATE dbinfo SET value = 0 WHERE key = 'ignore_items_update_triggers'").Error; err != nil {
		return fmt.Errorf("enabling Launchpad update triggers failed: %w", err)
	}

	utils.Indent(log.WithField("count", len(triggers)).Info, 3)("Restoring SQLite Triggers")
	for _, trigger := range triggers {
		if strings.TrimSpace(trigger.SQL) == "" {
			continue
		}
		if err := tx.Exec(trigger.SQL).Error; err != nil {
			return fmt.Errorf("Restoring SQLite Trigger %q Failed: %w", trigger.Name, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("Committing Repair Transaction Failed: %w", err)
	}
	committed = true

	if err := lp.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return fmt.Errorf("Checkpointing SQLite WAL Failed: %w", err)
	}
	if err := lp.DB.Exec("PRAGMA optimize").Error; err != nil {
		return fmt.Errorf("Optimizing SQLite Database Failed: %w", err)
	}

	utils.Indent(log.WithField("apps", len(placedAppIDs)).Info, 3)("Repair Complete")
	return nil
}

func nonEmptyRepairPages(pages []Page) []Page {
	contentPages := make([]Page, 0, len(pages))
	for _, page := range pages {
		if len(page.Items) == 0 {
			continue
		}
		contentPages = append(contentPages, page)
	}
	return contentPages
}

func getNextRepairID(tx *gorm.DB) (func() int, error) {
	var current int
	if err := tx.Raw("SELECT IFNULL(MAX(rowid), 6) FROM items").Scan(&current).Error; err != nil {
		return nil, fmt.Errorf("getting max item ID failed: %w", err)
	}

	return func() int {
		current++
		return current
	}, nil
}

func insertRepairItem(tx *gorm.DB, rowID, itemType, parentID, ordering, flags int, itemUUID string) error {
	if itemUUID == "" {
		itemUUID = strings.ToUpper(uuid.New().String())
	}

	if err := tx.Exec(`
		INSERT INTO items (rowid, uuid, flags, type, parent_id, ordering)
		VALUES (?, ?, ?, ?, ?, ?)
	`, rowID, itemUUID, flags, itemType, parentID, ordering).Error; err != nil {
		return fmt.Errorf("inserting item %d failed: %w", rowID, err)
	}

	return nil
}

func insertRepairGroup(tx *gorm.DB, itemID int, title string) error {
	var err error
	if title == "" {
		err = tx.Exec("INSERT INTO groups (item_id, title) VALUES (?, NULL)", itemID).Error
	} else {
		err = tx.Exec("INSERT INTO groups (item_id, title) VALUES (?, ?)", itemID, title).Error
	}
	if err != nil {
		return fmt.Errorf("inserting group %d failed: %w", itemID, err)
	}
	return nil
}

func findRepairAppIDs(tx *gorm.DB, title string) ([]int, error) {
	var rows []appIDRow
	if err := tx.Raw(`
		SELECT apps.item_id
		FROM apps
		JOIN items ON items.rowid = apps.item_id
		WHERE apps.title = ?
		ORDER BY apps.item_id
	`, title).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("finding app %q failed: %w", title, err)
	}

	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func hideUnplacedRepairApps(tx *gorm.DB, placedAppIDs map[int]bool) error {
	if len(placedAppIDs) == 0 {
		return nil
	}

	ids := make([]int, 0, len(placedAppIDs))
	for id := range placedAppIDs {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	query := fmt.Sprintf(`
		UPDATE items
		SET parent_id = 6, ordering = -1, type = ?
		WHERE type = ?
		AND rowid NOT IN (%s)
	`, placeholders(len(ids)))

	args := make([]any, 0, len(ids)+2)
	args = append(args, ApplicationType, ApplicationType)
	for _, id := range ids {
		args = append(args, id)
	}

	if err := tx.Exec(query, args...).Error; err != nil {
		return fmt.Errorf("hiding apps not listed in YAML failed: %w", err)
	}

	return nil
}

func removeLiteralOtherRepairFolders(tx *gorm.DB) error {
	var otherGroups []groupIDRow
	if err := tx.Raw("SELECT item_id FROM groups WHERE title = ?", "Other").Scan(&otherGroups).Error; err != nil {
		return fmt.Errorf("finding literal Other folders failed: %w", err)
	}

	for _, otherGroup := range otherGroups {
		var pageRows []itemIDRow
		if err := tx.Raw(`
			SELECT rowid
			FROM items
			WHERE parent_id = ?
			AND type = ?
		`, otherGroup.ID, PageType).Scan(&pageRows).Error; err != nil {
			return fmt.Errorf("finding literal Other folder pages failed: %w", err)
		}

		pageIDs := make([]int, 0, len(pageRows))
		for _, row := range pageRows {
			pageIDs = append(pageIDs, row.ID)
			if err := tx.Exec(`
				UPDATE items
				SET parent_id = 6, ordering = -1, type = ?
				WHERE parent_id = ?
				AND type = ?
			`, ApplicationType, row.ID, ApplicationType).Error; err != nil {
				return fmt.Errorf("hiding apps from literal Other folder failed: %w", err)
			}
		}

		if len(pageIDs) > 0 {
			args := make([]any, 0, len(pageIDs))
			for _, id := range pageIDs {
				args = append(args, id)
			}

			if err := tx.Exec(fmt.Sprintf("DELETE FROM groups WHERE item_id IN (%s)", placeholders(len(pageIDs))), args...).Error; err != nil {
				return fmt.Errorf("deleting literal Other page groups failed: %w", err)
			}
			if err := tx.Exec(fmt.Sprintf("DELETE FROM items WHERE rowid IN (%s)", placeholders(len(pageIDs))), args...).Error; err != nil {
				return fmt.Errorf("deleting literal Other pages failed: %w", err)
			}
		}

		if err := tx.Exec("DELETE FROM groups WHERE item_id = ?", otherGroup.ID).Error; err != nil {
			return fmt.Errorf("deleting literal Other group failed: %w", err)
		}
		if err := tx.Exec("DELETE FROM items WHERE rowid = ?", otherGroup.ID).Error; err != nil {
			return fmt.Errorf("deleting literal Other item failed: %w", err)
		}
	}

	return nil
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func escapeSQLiteIdentifier(identifier string) string {
	return strings.ReplaceAll(identifier, `"`, `""`)
}
