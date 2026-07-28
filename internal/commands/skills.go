package commands

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	bundledskills "github.com/maquina/recuerd0-cli/skills"
)

type skillCatalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Files       int    `json:"files"`
	root        string
}

type skillListData struct {
	Skills []skillCatalogEntry `json:"skills"`
}

type installedSkill struct {
	Name  string   `json:"name"`
	Path  string   `json:"path"`
	Files []string `json:"files"`
}

type skillInstallData struct {
	Installed []installedSkill `json:"installed"`
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List and install bundled agent skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List bundled agent skills",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		catalog, err := loadSkillCatalog()
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("loading bundled skills: %v", err)))
			return
		}

		printSuccessWithSummary(
			skillListData{Skills: catalog},
			fmt.Sprintf("%d skill(s) available", len(catalog)),
		)
	},
}

var (
	skillsInstallGlobal bool
	skillsInstallTarget string
	skillsInstallForce  bool
)

var skillsInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install bundled agent skills for Claude Code",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if skillsInstallGlobal && skillsInstallTarget != "" {
			exitWithError(errors.NewInvalidArgsError("--global and --target cannot be used together"))
			return
		}

		catalog, err := loadSkillCatalog()
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("loading bundled skills: %v", err)))
			return
		}

		selected := catalog
		if len(args) == 1 {
			selected = nil
			for _, skill := range catalog {
				if skill.Name == args[0] {
					selected = append(selected, skill)
					break
				}
			}
			if len(selected) == 0 {
				exitWithError(errors.NewInvalidArgsError(fmt.Sprintf("unknown skill %q", args[0])))
				return
			}
		}

		target, err := resolveSkillsTarget()
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("resolving skill target: %v", err)))
			return
		}

		destinations := make([]string, len(selected))
		for i, skill := range selected {
			destination, err := validatedSkillDestination(target, skill.Name)
			if err != nil {
				exitWithError(errors.NewError(fmt.Sprintf("validating skill destination: %v", err)))
				return
			}
			destinations[i] = destination

			if _, err := os.Lstat(destination); err == nil {
				if !skillsInstallForce {
					exitWithError(errors.NewError(fmt.Sprintf("skill already installed at %s — re-run with --force to overwrite", destination)))
					return
				}
			} else if !os.IsNotExist(err) {
				exitWithError(errors.NewError(fmt.Sprintf("checking skill destination: %v", err)))
				return
			}
		}

		installed := make([]installedSkill, 0, len(selected))
		for i, skill := range selected {
			destination := destinations[i]
			if skillsInstallForce {
				if err := os.RemoveAll(destination); err != nil {
					exitWithError(errors.NewError(fmt.Sprintf("removing installed skill: %v", err)))
					return
				}
			}

			files, err := copyEmbeddedSkill(skill, destination)
			if err != nil {
				exitWithError(errors.NewError(fmt.Sprintf("installing skill %q: %v", skill.Name, err)))
				return
			}
			installed = append(installed, installedSkill{
				Name:  skill.Name,
				Path:  destination,
				Files: files,
			})
		}

		printSuccessWithSummary(
			skillInstallData{Installed: installed},
			fmt.Sprintf("%d skill(s) installed", len(installed)),
		)
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)

	skillsInstallCmd.Flags().BoolVar(&skillsInstallGlobal, "global", false, "install to $HOME/.claude/skills")
	skillsInstallCmd.Flags().StringVar(&skillsInstallTarget, "target", "", "install to a custom skills directory")
	skillsInstallCmd.Flags().BoolVar(&skillsInstallForce, "force", false, "replace an existing installed skill")
	skillsCmd.AddCommand(skillsInstallCmd)
}

func loadSkillCatalog() ([]skillCatalogEntry, error) {
	entries, err := fs.ReadDir(bundledskills.FS, ".")
	if err != nil {
		return nil, err
	}

	catalog := make([]skillCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		root := entry.Name()
		skillFile := path.Join(root, "SKILL.md")
		data, err := fs.ReadFile(bundledskills.FS, skillFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", skillFile, err)
		}

		name, description, err := parseSkillFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", skillFile, err)
		}
		if name != root {
			return nil, fmt.Errorf("%s name %q does not match directory %q", skillFile, name, root)
		}

		files := 0
		if err := fs.WalkDir(bundledskills.FS, root, func(_ string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if dirEntry.IsDir() {
				return nil
			}
			info, err := dirEntry.Info()
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				files++
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walking %s: %w", root, err)
		}

		catalog = append(catalog, skillCatalogEntry{
			Name:        name,
			Description: description,
			Files:       files,
			root:        root,
		})
	}

	sort.Slice(catalog, func(i, j int) bool {
		return catalog[i].Name < catalog[j].Name
	})
	return catalog, nil
}

func parseSkillFrontmatter(data []byte) (string, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() || scanner.Text() != "---" {
		return "", "", fmt.Errorf("missing opening frontmatter delimiter")
	}

	var name string
	var description string
	closed := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			closed = true
			break
		}
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
		if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if !closed {
		return "", "", fmt.Errorf("missing closing frontmatter delimiter")
	}
	if name == "" {
		return "", "", fmt.Errorf("missing name")
	}
	if description == "" {
		return "", "", fmt.Errorf("missing description")
	}
	return name, description, nil
}

func resolveSkillsTarget() (string, error) {
	var target string
	switch {
	case skillsInstallTarget != "":
		target = skillsInstallTarget
	case skillsInstallGlobal:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		target = filepath.Join(home, ".claude", "skills")
	default:
		target = filepath.Join(".", ".claude", "skills")
	}
	return filepath.Abs(target)
}

func validatedSkillDestination(target, name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid skill name %q", name)
	}

	target = filepath.Clean(target)
	destination := filepath.Join(target, name)
	relative, err := filepath.Rel(target, destination)
	if err != nil {
		return "", err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("skill destination %q escapes target %q", destination, target)
	}
	return destination, nil
}

func copyEmbeddedSkill(skill skillCatalogEntry, destination string) ([]string, error) {
	if err := makeDirectories(destination, 0755); err != nil {
		return nil, err
	}

	files := make([]string, 0, skill.Files)
	err := fs.WalkDir(bundledskills.FS, skill.root, func(embeddedPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if embeddedPath == skill.root {
			return nil
		}
		relative := strings.TrimPrefix(embeddedPath, skill.root+"/")
		if relative == embeddedPath {
			return fmt.Errorf("embedded path %q is outside skill root %q", embeddedPath, skill.root)
		}

		hostPath := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			return makeDirectories(hostPath, 0755)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported embedded file type at %s", embeddedPath)
		}

		data, err := fs.ReadFile(bundledskills.FS, embeddedPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(hostPath, data, 0644); err != nil {
			return err
		}
		if err := os.Chmod(hostPath, 0644); err != nil {
			return err
		}

		files = append(files, path.Clean(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func makeDirectories(directory string, mode fs.FileMode) error {
	directory = filepath.Clean(directory)
	var missing []string
	current := directory

	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}

		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("no existing parent for %s", directory)
		}
		current = parent
	}

	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], mode); err != nil {
			if !os.IsExist(err) {
				return err
			}
			info, statErr := os.Stat(missing[i])
			if statErr != nil {
				return statErr
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", missing[i])
			}
			continue
		}
		if err := os.Chmod(missing[i], mode); err != nil {
			return err
		}
	}
	return nil
}
