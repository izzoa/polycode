package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestSkill(t *testing.T, dir, name, desc string, tools []ToolManifest) string {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := "name: " + name + "\ndescription: " + desc + "\nversion: \"1.0\"\ncommand: " + name + "\n"
	if len(tools) > 0 {
		manifest += "tools:\n"
		for _, tool := range tools {
			manifest += "  - name: " + tool.Name + "\n"
			manifest += "    description: " + tool.Description + "\n"
			manifest += "    handler: " + tool.Handler + "\n"
		}
	}

	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	return skillDir
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	skillDir := createTestSkill(t, dir, "test-skill", "A test skill", nil)

	m, err := LoadManifest(filepath.Join(skillDir, "skill.yaml"))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", m.Name, "test-skill")
	}
	if m.Description != "A test skill" {
		t.Errorf("Description = %q, want %q", m.Description, "A test skill")
	}
	if m.Version != "1.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0")
	}
	if m.Command != "test-skill" {
		t.Errorf("Command = %q, want %q", m.Command, "test-skill")
	}
}

func TestLoadManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte("description: no name"), 0644)

	_, err := LoadManifest(filepath.Join(dir, "skill.yaml"))
	if err == nil {
		t.Fatal("expected error for manifest without name")
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest("/nonexistent/skill.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system_prompt.md"), []byte("Be helpful."), 0644)

	got := LoadSystemPrompt(dir)
	if got != "Be helpful." {
		t.Errorf("SystemPrompt = %q, want %q", got, "Be helpful.")
	}
}

func TestLoadSystemPrompt_Missing(t *testing.T) {
	got := LoadSystemPrompt(t.TempDir())
	if got != "" {
		t.Errorf("expected empty string for missing system_prompt.md, got %q", got)
	}
}

func TestRegistryLoad(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "alpha", "First skill", nil)
	createTestSkill(t, dir, "beta", "Second skill", nil)

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	skills := reg.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Manifest.Name != "alpha" {
		t.Errorf("first skill = %q, want %q", skills[0].Manifest.Name, "alpha")
	}
	if skills[1].Manifest.Name != "beta" {
		t.Errorf("second skill = %q, want %q", skills[1].Manifest.Name, "beta")
	}
}

func TestRegistryLoad_EmptyDir(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Error("expected 0 skills in empty dir")
	}
}

func TestRegistryLoad_NonexistentDir(t *testing.T) {
	reg := NewRegistry("/nonexistent/path")
	if err := reg.Load(); err != nil {
		t.Fatalf("Load should not error on missing dir: %v", err)
	}
}

func TestRegistryGet(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "find-me", "Findable", nil)

	reg := NewRegistry(dir)
	reg.Load()

	got := reg.Get("find-me")
	if got == nil {
		t.Fatal("expected to find skill")
	}
	if got.Manifest.Name != "find-me" {
		t.Errorf("Name = %q, want %q", got.Manifest.Name, "find-me")
	}

	if reg.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent skill")
	}
}

func TestRegistryInstallAndRemove(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create a skill to install
	createTestSkill(t, sourceDir, "new-skill", "A new skill", nil)

	reg := NewRegistry(skillsDir)
	reg.Load()

	// Install
	if err := reg.Install(filepath.Join(sourceDir, "new-skill")); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if reg.Get("new-skill") == nil {
		t.Fatal("skill should be available after install")
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(skillsDir, "new-skill", "skill.yaml")); err != nil {
		t.Fatalf("skill.yaml should exist after install: %v", err)
	}

	// Remove
	if err := reg.Remove("new-skill"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if reg.Get("new-skill") != nil {
		t.Error("skill should be nil after remove")
	}
}

func TestRegistryInstall_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "dupe", "First", nil)

	sourceDir := t.TempDir()
	createTestSkill(t, sourceDir, "dupe", "Second", nil)

	reg := NewRegistry(dir)
	reg.Load()

	err := reg.Install(filepath.Join(sourceDir, "dupe"))
	if err == nil {
		t.Fatal("expected error for duplicate skill name")
	}
}

func TestRegistryRemove_Nonexistent(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	err := reg.Remove("ghost")
	if err == nil {
		t.Fatal("expected error for removing nonexistent skill")
	}
}

func TestToToolDefinitions(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "myskill", "Skill with tools", []ToolManifest{
		{Name: "greet", Description: "Say hello", Handler: "echo hello"},
		{Name: "count", Description: "Count things", Handler: "wc -l"},
	})

	reg := NewRegistry(dir)
	reg.Load()

	defs := reg.ToToolDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 tool definitions, got %d", len(defs))
	}

	if defs[0].Name != "skill_myskill_greet" {
		t.Errorf("tool name = %q, want %q", defs[0].Name, "skill_myskill_greet")
	}
	if defs[1].Name != "skill_myskill_count" {
		t.Errorf("tool name = %q, want %q", defs[1].Name, "skill_myskill_count")
	}
}

func TestExecuteTool(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "echo-skill", "Echo skill", []ToolManifest{
		{Name: "echo", Description: "Echo input", Handler: "cat"},
	})

	reg := NewRegistry(dir)
	reg.Load()

	out, err := reg.ExecuteTool(context.Background(), "skill_echo-skill_echo", `{"message":"hello"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", out)
	}
}

func TestExecuteTool_UnknownSkill(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	_, err := reg.ExecuteTool(context.Background(), "skill_unknown_tool", "{}")
	if err == nil {
		t.Fatal("expected error for unknown skill tool")
	}
}

func TestSystemPrompts(t *testing.T) {
	dir := t.TempDir()
	skillDir := createTestSkill(t, dir, "prompted", "Has prompt", nil)
	os.WriteFile(filepath.Join(skillDir, "system_prompt.md"), []byte("Be creative."), 0644)

	reg := NewRegistry(dir)
	reg.Load()

	prompts := reg.SystemPrompts()
	if !strings.Contains(prompts, "Be creative.") {
		t.Errorf("expected system prompts to contain skill prompt, got %q", prompts)
	}
}

func TestSlashCommands(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "review", "Review skill", nil)
	createTestSkill(t, dir, "deploy", "Deploy skill", nil)

	reg := NewRegistry(dir)
	reg.Load()

	cmds := reg.SlashCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 slash commands, got %d", len(cmds))
	}
}

func TestHandleCommand(t *testing.T) {
	dir := t.TempDir()
	skillDir := createTestSkill(t, dir, "review", "Review skill", nil)
	os.WriteFile(filepath.Join(skillDir, "system_prompt.md"), []byte("Review code carefully."), 0644)

	reg := NewRegistry(dir)
	reg.Load()

	prompt, ok := reg.HandleCommand("/review")
	if !ok {
		t.Fatal("expected command to be handled")
	}
	if !strings.Contains(prompt, "Review code carefully.") {
		t.Errorf("expected system prompt, got %q", prompt)
	}

	_, ok = reg.HandleCommand("/nonexistent")
	if ok {
		t.Error("expected unknown command to not be handled")
	}
}

func TestFormatList(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "alpha", "First skill", []ToolManifest{
		{Name: "tool1", Description: "Tool one", Handler: "echo"},
	})

	reg := NewRegistry(dir)
	reg.Load()

	output := reg.FormatList()
	if !strings.Contains(output, "alpha") {
		t.Error("expected listing to contain skill name")
	}
	if !strings.Contains(output, "v1.0") {
		t.Error("expected listing to contain version")
	}
	if !strings.Contains(output, "tools: 1") {
		t.Error("expected listing to contain tool count")
	}
}

func TestFormatList_Empty(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	output := reg.FormatList()
	if !strings.Contains(output, "No skills installed") {
		t.Errorf("expected empty message, got %q", output)
	}
}
