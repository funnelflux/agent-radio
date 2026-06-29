package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workspaces []Workspace `yaml:"workspaces"`
}

type Workspace struct {
	Name         string       `yaml:"name"`
	Description  string       `yaml:"description,omitempty"`
	Root         string       `yaml:"root"`
	Color        string       `yaml:"color,omitempty"`
	Icon         string       `yaml:"icon,omitempty"`
	Tags         []string     `yaml:"tags,omitempty"`
	Capabilities []string     `yaml:"capabilities,omitempty"`
	Repositories []Repository `yaml:"repositories,omitempty"`
	Sessions     []Session    `yaml:"sessions"`
}

type Repository struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	Path         string   `yaml:"path"`
	Role         string   `yaml:"role,omitempty"`
	Product      string   `yaml:"product,omitempty"`
	Owner        string   `yaml:"owner,omitempty"`
	Provides     []string `yaml:"provides,omitempty"`
	Consumes     []string `yaml:"consumes,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
}

type Session struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description,omitempty"`
	Type         string   `yaml:"type,omitempty"`
	RepoID       string   `yaml:"repo_id,omitempty"`
	Path         string   `yaml:"path"`
	Command      string   `yaml:"command"`
	AgentID      string   `yaml:"agent_id,omitempty"`
	Icon         string   `yaml:"icon,omitempty"`
	Color        string   `yaml:"color,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty"`
}

func DefaultPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("AGENT_RADIO_CONFIG")); v != "" {
		return v, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent-radio", "config.yaml"), nil
}

func LoadDefault() (Config, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, "", err
	}
	cfg, err := Load(path)
	return cfg, path, err
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.expand()
	return cfg, nil
}

func (c Config) Validate() error {
	if len(c.Workspaces) == 0 {
		return errors.New("config has no workspaces")
	}
	repoIDs := map[string]struct{}{}
	for wi, ws := range c.Workspaces {
		if strings.TrimSpace(ws.Name) == "" {
			return fmt.Errorf("workspace %d has empty name", wi+1)
		}
		for ri, repo := range ws.Repositories {
			if strings.TrimSpace(repo.ID) == "" {
				return fmt.Errorf("workspace %q repository %d has empty id", ws.Name, ri+1)
			}
			if _, ok := repoIDs[repo.ID]; ok {
				return fmt.Errorf("duplicate repository id %q", repo.ID)
			}
			repoIDs[repo.ID] = struct{}{}
			if strings.TrimSpace(repo.Path) == "" {
				return fmt.Errorf("repository %q has empty path", repo.ID)
			}
		}
	}
	sessionNames := map[string]struct{}{}
	for wi, ws := range c.Workspaces {
		if strings.TrimSpace(ws.Name) == "" {
			return fmt.Errorf("workspace %d has empty name", wi+1)
		}
		if len(ws.Sessions) == 0 {
			return fmt.Errorf("workspace %q has no sessions", ws.Name)
		}
		for si, s := range ws.Sessions {
			if strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("workspace %q session %d has empty name", ws.Name, si+1)
			}
			if _, ok := sessionNames[s.Name]; ok {
				return fmt.Errorf("duplicate session name %q", s.Name)
			}
			sessionNames[s.Name] = struct{}{}
			if strings.TrimSpace(s.Path) == "" {
				return fmt.Errorf("session %q has empty path", s.Name)
			}
			if strings.TrimSpace(s.Command) == "" {
				return fmt.Errorf("session %q has empty command", s.Name)
			}
			if strings.TrimSpace(s.RepoID) != "" {
				if _, ok := repoIDs[s.RepoID]; !ok {
					return fmt.Errorf("session %q references unknown repo_id %q", s.Name, s.RepoID)
				}
			}
		}
	}
	return nil
}

func (c *Config) expand() {
	for wi := range c.Workspaces {
		c.Workspaces[wi].Root = expandPath(c.Workspaces[wi].Root)
		for ri := range c.Workspaces[wi].Repositories {
			repo := &c.Workspaces[wi].Repositories[ri]
			repo.Path = expandPath(repo.Path)
			if strings.TrimSpace(repo.Name) == "" {
				repo.Name = repo.ID
			}
		}
		for si := range c.Workspaces[wi].Sessions {
			s := &c.Workspaces[wi].Sessions[si]
			s.Path = expandPath(s.Path)
			if strings.TrimSpace(s.AgentID) == "" {
				s.AgentID = s.Name
			}
		}
	}
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
