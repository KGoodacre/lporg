package database

import (
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/blacktop/lporg/internal/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v3"
)

// Config Is The Launchpad Config
type Config struct {
	Apps    Apps    `yaml:"apps" json:"apps,omitempty"`
	Widgets Apps    `yaml:"widgets" json:"widgets,omitempty"`
	Dock    Dock    `yaml:"dock_items" json:"dock_items,omitempty"  mapstructure:"dock_items"`
	Desktop Desktop `yaml:"desktop" json:"desktop,omitempty"  mapstructure:"desktop"`
}

// GetFolderContainingApp Returns The Folder Name That Contains The App
func (c Config) GetFolderContainingApp(app string) (string, error) {
	for _, page := range c.Apps.Pages {
		for _, item := range page.Items {
			switch item.(type) {
			case string:
				continue
			default:
				var folder AppFolder
				if err := mapstructure.Decode(item, &folder); err != nil {
					return "", errors.Wrap(err, "mapstructure unable to decode config folder")
				}
				for _, page := range folder.Pages {
					for _, item := range page.Items {
						if item == app {
							return folder.Name, nil
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("unable to find folder containing app %s", app)
}

// Verify That The Config Is Valid
func (c Config) Verify() error {
	for _, page := range c.Apps.Pages {
		for _, item := range page.Items {
			switch item.(type) {
			case string:
				continue
			default:
				var folder AppFolder
				if err := mapstructure.Decode(item, &folder); err != nil {
					return fmt.Errorf("mapstructure unable to decode config folder")
				}
				if len(folder.Pages) > 0 {
					if len(folder.Pages[0].Items) == 0 { // verify that all folders contain at least 1 item
						return fmt.Errorf("Folder %s Must Contain At Least 1 Item To Be Valid", folder.Name)
					}
				}
			}
		}
	}
	return nil
}

// Apps Is The Launchpad Apps Config Object
type Apps struct {
	Pages []Page `yaml:"pages" json:"pages,omitempty"`
}

// Page Is A Launchpad Page Object
type Page struct {
	Number int   `yaml:"number" json:"number"`
	Items  []any `yaml:"items,omitempty" json:"items,omitempty"`
}

// AppFolder Is A Launchpad Folder Object
type AppFolder struct {
	Name  string       `yaml:"folder" json:"folder,omitempty" mapstructure:"folder"`
	Pages []FolderPage `yaml:"pages,omitempty" json:"pages,omitempty"`
}

// FolderPage Is A Launchpad Folder Page Object
type FolderPage struct {
	Number int      `yaml:"number,omitempty" json:"number"`
	Items  []string `yaml:"items,omitempty" json:"items,omitempty"`
}

// Desktop Is The Desktop Object
type Desktop struct {
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
}

type FolderDisplay int

const (
	stack  FolderDisplay = 0
	folder FolderDisplay = 1
)

type FolderView int

const (
	auto FolderView = 0
	fan  FolderView = 1
	grid FolderView = 2
	list FolderView = 3
)

type FolderSort int

const (
	name         FolderSort = 1
	dateadded    FolderSort = 2
	datemodified FolderSort = 3
	datecreated  FolderSort = 4
	kind         FolderSort = 5
)

// Folder Is A Launchpad Folder Object
type Folder struct {
	Path    string        `yaml:"path,omitempty" json:"path,omitempty"`
	Display FolderDisplay `yaml:"display,omitempty" json:"display,omitempty"`
	View    FolderView    `yaml:"view,omitempty" json:"view,omitempty"`
	Sort    FolderSort    `yaml:"sort,omitempty" json:"sort,omitempty"`
}

// DockSettings Is The Launchpad Dock Settings Object
type DockSettings struct {
	AutoHide              bool `yaml:"autohide" json:"autohide,omitempty"`
	LargeSize             any  `yaml:"largesize" json:"largesize,omitempty"`
	Magnification         bool `yaml:"magnification" json:"magnification,omitempty"`
	MinimizeToApplication bool `yaml:"minimize-to-application" json:"minimize-to-application,omitempty"`
	MruSpaces             bool `yaml:"mru-spaces" json:"mru-spaces,omitempty"`
	ShowRecents           bool `yaml:"show-recents" json:"show-recents,omitempty"`
	TileSize              any  `yaml:"tilesize" json:"tilesize,omitempty"`
}

// Dock Is The Launchpad Dock Config Object
type Dock struct {
	Apps     []string      `yaml:"apps,omitempty" json:"apps,omitempty"`
	Others   []Folder      `yaml:"others,omitempty" json:"others,omitempty"`
	Settings *DockSettings `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// LoadConfig Loads The Launchpad Config From The Config File
func LoadConfig(filename string) (Config, error) {
	var conf Config

	utils.Indent(log.WithField("path", filename).Info, 2)("Parsing Launchpad Config YAML")
	data, err := os.ReadFile(filename)
	if err != nil {
		utils.Indent(log.WithError(err).WithField("path", filename).Fatal, 3)("Config File Not Found")
		return conf, err
	}

	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		utils.Indent(log.WithError(err).WithField("path", filename).Fatal, 3)("Unmarshalling YAML Failed")
		return conf, err
	}

	if err := conf.Verify(); err != nil {
		return conf, fmt.Errorf("Config Verification Failed: %v", err)
	}

	return conf, nil
}
