package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
	bundledskills "github.com/maquina/recuerd0-cli/skills"
)

func resetSkillsInstallFlags(t *testing.T) {
	t.Helper()
	skillsInstallGlobal = false
	skillsInstallTarget = ""
	skillsInstallForce = false
	t.Cleanup(func() {
		skillsInstallGlobal = false
		skillsInstallTarget = ""
		skillsInstallForce = false
	})
}

func embeddedSkillFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := fs.WalkDir(bundledskills.FS, root, func(embeddedPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files = append(files, strings.TrimPrefix(embeddedPath, root+"/"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded skill: %v", err)
	}
	sort.Strings(files)
	return files
}

func assertInstalledSkillMatchesEmbedded(t *testing.T, destination, embeddedRoot string) {
	t.Helper()
	expectedFiles := embeddedSkillFiles(t, embeddedRoot)
	var actualFiles []string

	rootInfo, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat installed skill: %v", err)
	}
	if got := rootInfo.Mode().Perm(); got != 0755 {
		t.Errorf("skill root mode = %04o, want 0755", got)
	}

	err = filepath.WalkDir(destination, func(hostPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(destination, hostPath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if got := info.Mode().Perm(); got != 0755 {
				t.Errorf("directory %s mode = %04o, want 0755", relative, got)
			}
			return nil
		}
		if got := info.Mode().Perm(); got != 0644 {
			t.Errorf("file %s mode = %04o, want 0644", relative, got)
		}

		slashRelative := filepath.ToSlash(relative)
		actualFiles = append(actualFiles, slashRelative)
		actual, err := os.ReadFile(hostPath)
		if err != nil {
			return err
		}
		expected, err := fs.ReadFile(bundledskills.FS, path.Join(embeddedRoot, slashRelative))
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s bytes differ from embedded file", slashRelative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk installed skill: %v", err)
	}
	sort.Strings(actualFiles)
	if !reflect.DeepEqual(actualFiles, expectedFiles) {
		t.Fatalf("installed files = %#v, want %#v", actualFiles, expectedFiles)
	}
}

func TestSkillsListUsesEmbeddedCatalog(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsListCmd.Run(skillsListCmd, nil)
	})

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
	if !result.Response.Success {
		t.Fatal("expected success response")
	}
	data, ok := result.Response.Data.(skillListData)
	if !ok {
		t.Fatalf("response data type = %T, want skillListData", result.Response.Data)
	}
	if len(data.Skills) != 1 {
		t.Fatalf("skills count = %d, want 1", len(data.Skills))
	}

	skillFile, err := fs.ReadFile(bundledskills.FS, "recuerd0/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	name, description, err := parseSkillFrontmatter(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	got := data.Skills[0]
	if got.Name != name {
		t.Errorf("name = %q, want %q", got.Name, name)
	}
	if got.Description != description {
		t.Errorf("description = %q, want %q", got.Description, description)
	}
	if want := len(embeddedSkillFiles(t, "recuerd0")); got.Files != want {
		t.Errorf("files = %d, want %d", got.Files, want)
	}
	if result.Response.Summary != "1 skill(s) available" {
		t.Errorf("summary = %q, want %q", result.Response.Summary, "1 skill(s) available")
	}
}

func TestSkillsInstallTargetReinstallAndForce(t *testing.T) {
	resetSkillsInstallFlags(t)
	target := t.TempDir()
	skillsInstallTarget = target
	stderr := setSkillsGuidance(t, true)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, []string{"recuerd0"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
	data, ok := result.Response.Data.(skillInstallData)
	if !ok {
		t.Fatalf("response data type = %T, want skillInstallData", result.Response.Data)
	}
	if len(data.Installed) != 1 {
		t.Fatalf("installed count = %d, want 1", len(data.Installed))
	}

	destination := filepath.Join(target, "recuerd0")
	absoluteDestination, err := filepath.Abs(destination)
	if err != nil {
		t.Fatal(err)
	}
	if data.Installed[0].Path != absoluteDestination {
		t.Errorf("installed path = %q, want %q", data.Installed[0].Path, absoluteDestination)
	}
	if result.Response.Location != target {
		t.Errorf("location = %q, want %q", result.Response.Location, target)
	}
	if result.Response.Summary != fmt.Sprintf("Installed recuerd0 to %s", target) {
		t.Errorf("summary = %q", result.Response.Summary)
	}
	if len(result.Response.Breadcrumbs) != 1 ||
		result.Response.Breadcrumbs[0].Cmd != "recuerd0 skills list" {
		t.Fatalf("breadcrumbs = %#v", result.Response.Breadcrumbs)
	}
	wantGuidance := fmt.Sprintf(skillsInstallGuidance, target)
	if stderr.String() != wantGuidance {
		t.Fatalf("guidance:\ngot:\n%s\nwant:\n%s", stderr.String(), wantGuidance)
	}
	wantFiles := embeddedSkillFiles(t, "recuerd0")
	if !reflect.DeepEqual(data.Installed[0].Files, wantFiles) {
		t.Errorf("response files = %#v, want %#v", data.Installed[0].Files, wantFiles)
	}
	assertInstalledSkillMatchesEmbedded(t, destination, "recuerd0")

	skillPath := filepath.Join(destination, "SKILL.md")
	changed := []byte("user-owned changed bytes\n")
	if err := os.WriteFile(skillPath, changed, 0644); err != nil {
		t.Fatal(err)
	}
	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, []string{"recuerd0"})
	})

	wantMessage := fmt.Sprintf("skill already installed at %s — re-run with --force to overwrite", absoluteDestination)
	if result.Response.Success {
		t.Fatal("expected reinstall error")
	}
	if result.Response.Error == nil || result.Response.Error.Message != wantMessage {
		t.Fatalf("error message = %#v, want %q", result.Response.Error, wantMessage)
	}
	untouched, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(untouched, changed) {
		t.Fatal("reinstall without --force changed the existing skill")
	}
	if stderr.String() != wantGuidance {
		t.Fatalf("existing-install failure wrote guidance: %q", stderr.String())
	}

	strayPath := filepath.Join(destination, "stray.txt")
	if err := os.WriteFile(strayPath, []byte("remove me"), 0644); err != nil {
		t.Fatal(err)
	}
	skillsInstallForce = true
	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, []string{"recuerd0"})
	})

	if result.ExitCode != 0 || !result.Response.Success {
		t.Fatalf("forced install failed: %#v", result.Response.Error)
	}
	if _, err := os.Lstat(strayPath); !os.IsNotExist(err) {
		t.Fatalf("stray file still exists after forced install: %v", err)
	}
	if stderr.String() != wantGuidance+wantGuidance {
		t.Fatalf("expected one guidance block per successful invocation: %q", stderr.String())
	}
	assertInstalledSkillMatchesEmbedded(t, destination, "recuerd0")
}

func TestSkillsInstallDefaultTargetUsesRelativeDisplayPath(t *testing.T) {
	resetSkillsInstallFlags(t)
	project := t.TempDir()
	t.Chdir(project)
	stderr := setSkillsGuidance(t, true)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, []string{"recuerd0"})
	})

	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("install failed: %#v", result)
	}
	if result.Response.Location != filepath.Join(".claude", "skills") {
		t.Fatalf("location = %q", result.Response.Location)
	}
	if result.Response.Summary != "Installed recuerd0 to .claude/skills" {
		t.Fatalf("summary = %q", result.Response.Summary)
	}
	want := fmt.Sprintf(skillsInstallGuidance, filepath.Join(".claude", "skills"))
	if stderr.String() != want {
		t.Fatalf("guidance:\ngot:\n%s\nwant:\n%s", stderr.String(), want)
	}
}

func TestSkillsInstallGlobalUsesIsolatedHome(t *testing.T) {
	resetSkillsInstallFlags(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillsInstallGlobal = true

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, nil)
	})

	if result.ExitCode != 0 || !result.Response.Success {
		t.Fatalf("global install failed: %#v", result.Response.Error)
	}
	assertInstalledSkillMatchesEmbedded(t, filepath.Join(home, ".claude", "skills", "recuerd0"), "recuerd0")
}

func TestSkillsInstallRejectsConflictingTargetFlags(t *testing.T) {
	resetSkillsInstallFlags(t)
	skillsInstallGlobal = true
	skillsInstallTarget = t.TempDir()

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, nil)
	})

	if result.ExitCode != clierrors.ExitInvalidArgs {
		t.Errorf("exit code = %d, want %d", result.ExitCode, clierrors.ExitInvalidArgs)
	}
	if result.Response.Error == nil || result.Response.Error.Code != clierrors.CodeInvalidArgs {
		t.Fatalf("error = %#v, want INVALID_ARGS", result.Response.Error)
	}
}

func TestSkillsInstallRejectsUnknownName(t *testing.T) {
	resetSkillsInstallFlags(t)
	skillsInstallTarget = t.TempDir()

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		skillsInstallCmd.Run(skillsInstallCmd, []string{"unknown"})
	})

	if result.ExitCode != clierrors.ExitInvalidArgs {
		t.Errorf("exit code = %d, want %d", result.ExitCode, clierrors.ExitInvalidArgs)
	}
	if result.Response.Error == nil || result.Response.Error.Code != clierrors.CodeInvalidArgs {
		t.Fatalf("error = %#v, want INVALID_ARGS", result.Response.Error)
	}
}

func TestSkillsConfigBypassUsesCommandAncestry(t *testing.T) {
	if !isSkillsCommand(skillsCmd) {
		t.Error("skills command should bypass config")
	}
	if !isSkillsCommand(skillsListCmd) {
		t.Error("skills list should bypass config")
	}
	if !isSkillsCommand(skillsInstallCmd) {
		t.Error("skills install should bypass config")
	}
	if isSkillsCommand(accountListCmd) {
		t.Error("unrelated list command should remain config-dependent")
	}

	unrelatedParent := &cobra.Command{Use: "unrelated"}
	unrelatedInstall := &cobra.Command{Use: "install"}
	unrelatedParent.AddCommand(unrelatedInstall)
	if isSkillsCommand(unrelatedInstall) {
		t.Error("unrelated install command should remain config-dependent")
	}
}

func TestSkillsBypassStillAppliesPrettyPrinting(t *testing.T) {
	previousPretty := cfgPretty
	cfgPretty = true
	t.Cleanup(func() {
		cfgPretty = previousPretty
		response.SetPrettyPrint(previousPretty)
	})

	rootCmd.PersistentPreRun(skillsListCmd, nil)
	data, err := response.Success(map[string]string{"ok": "yes"}).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n") {
		t.Fatalf("pretty output was not enabled before skills config bypass: %s", data)
	}
}

func TestSkillsInstallRootCommandDoesNotLoadConfig(t *testing.T) {
	if os.Getenv("RECUERD0_SKILLS_SUBPROCESS") == "1" {
		rootCmd.SetArgs([]string{"skills", "install"})
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	project := t.TempDir()
	home := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestSkillsInstallRootCommandDoesNotLoadConfig$")
	command.Dir = project
	command.Env = append(
		os.Environ(),
		"RECUERD0_SKILLS_SUBPROCESS=1",
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, output)
	}

	var envelope response.Response
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode subprocess output: %v\n%s", err, output)
	}
	if !envelope.Success {
		t.Fatalf("subprocess response was not successful: %s", output)
	}
	assertInstalledSkillMatchesEmbedded(t, filepath.Join(project, ".claude", "skills", "recuerd0"), "recuerd0")

	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("skills command created files under HOME: %#v", entries)
	}
}

func setSkillsGuidance(t *testing.T, tty bool) *bytes.Buffer {
	t.Helper()
	output := &bytes.Buffer{}
	oldOutput := skillsGuidanceOutput
	oldIsTTY := skillsGuidanceIsTTY
	skillsGuidanceOutput = output
	skillsGuidanceIsTTY = func() bool { return tty }
	t.Cleanup(func() {
		skillsGuidanceOutput = oldOutput
		skillsGuidanceIsTTY = oldIsTTY
	})
	return output
}
