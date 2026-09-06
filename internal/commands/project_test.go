package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
)

func TestChooseProject_TooManyArgs(t *testing.T) {
	cmd := parsedCmd(t, "switch", nil, "a", "b")
	if _, err := chooseProject(context.Background(), cmd); err == nil {
		t.Error("chooseProject() error = nil, want usage error")
	}
}

func TestChooseProject_KnownIDAdoptsItsWorkspace(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", WorkspaceID: "w1"}})
	})
	cmd := parsedCmd(t, "switch", nil, "p1")

	got, err := chooseProject(context.Background(), cmd)
	if err != nil {
		t.Fatalf("chooseProject() error = %v", err)
	}
	if got.Name != "Website" || got.WorkspaceID != "w1" {
		t.Errorf("chooseProject() = %+v, want {Name: Website, WorkspaceID: w1}", got)
	}
}

func TestChooseProject_UnknownIDFallsBackToBareID(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website"}})
	})
	cmd := parsedCmd(t, "switch", nil, "missing")

	got, err := chooseProject(context.Background(), cmd)
	if err != nil {
		t.Fatalf("chooseProject() error = %v", err)
	}
	if got.ID != "missing" || got.Name != "" {
		t.Errorf("chooseProject() = %+v, want bare {ID: missing}", got)
	}
}

func TestRunProjectList_Quiet(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1"}, {ID: "p2"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if out != "p1\np2\n" {
		t.Errorf("stdout = %q, want %q", out, "p1\np2\n")
	}
}

func TestRunProjectList_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"p1"`) {
		t.Errorf("stdout = %q, want raw JSON with id p1", out)
	}
}

func TestRunProjectList_TableTruncatesDescription(t *testing.T) {
	long := strings.Repeat("a", 100)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", Description: long}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if !strings.Contains(out, "…") || strings.Contains(out, long) {
		t.Errorf("stdout = %q, want the description truncated to 60 runes with an ellipsis", out)
	}
}

func TestRunProjectSwitch_NoArgsRequiresWorkspace(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.WorkspaceID = "" })
	cmd := parsedCmd(t, "switch", nil)
	if err := runProjectSwitch(context.Background(), cmd); err == nil {
		t.Error("runProjectSwitch() error = nil, want a missing-workspace error")
	}
}

type projectContextFile struct {
	Context struct {
		ProjectID   string `toml:"project_id"`
		WorkspaceID string `toml:"workspace_id"`
	} `toml:"context"`
}

func readProjectContextFile(t *testing.T, path string) projectContextFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var cfg projectContextFile
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return cfg
}

func TestRunProjectSwitch_ExplicitIDAdoptsWorkspace(t *testing.T) {
	path := withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", WorkspaceID: "w1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false })
	cmd := parsedCmd(t, "switch", nil, "p1")
	out := captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	if out != "Project set to Website\n" {
		t.Errorf("stdout = %q, want %q", out, "Project set to Website\n")
	}
	cfg := readProjectContextFile(t, path)
	if cfg.Context.ProjectID != "p1" || cfg.Context.WorkspaceID != "w1" {
		t.Errorf("config = %+v, want project=p1 workspace=w1 (adopted from the resolved project)", cfg.Context)
	}
}

func TestRunProjectSwitch_UnknownIDDoesNotAdoptWorkspace(t *testing.T) {
	path := withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{})
	})
	cmd := parsedCmd(t, "switch", nil, "missing")
	captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	cfg := readProjectContextFile(t, path)
	if cfg.Context.ProjectID != "missing" || cfg.Context.WorkspaceID != "" {
		t.Errorf("config = %+v, want project=missing and workspace left unset", cfg.Context)
	}
}

func TestRunProjectSwitch_QuietSuppressesMessage(t *testing.T) {
	withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true })
	cmd := parsedCmd(t, "switch", nil, "p1")
	out := captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}

// projectCreateFlags returns the flags declared by `project create` so tests
// parse the same flag set the real command does.
func projectCreateFlags() []cli.Flag {
	for _, c := range projectCommand.Commands {
		if c.Name == "create" {
			return c.Flags
		}
	}
	return nil
}

func TestRunProjectCreate_SetsContextAndPrints(t *testing.T) {
	path := withConfigFile(t)
	var gotBody map[string]string
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.Project{ID: "p1", Name: "Website", WorkspaceID: "w1"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false; o.JSON = false; o.WorkspaceID = "w1" })
	cmd := parsedCmd(t, "create", projectCreateFlags(), "-d", "Marketing site", "Website")
	out := captureStdout(t, func() {
		if err := runProjectCreate(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectCreate() error = %v", err)
		}
	})
	if out != "Project Website created and set as current.\n" {
		t.Errorf("stdout = %q, want the created-and-set message", out)
	}
	if gotBody["workspace_id"] != "w1" || gotBody["name"] != "Website" || gotBody["description"] != "Marketing site" {
		t.Errorf("request body = %v, want workspace_id=w1 name=Website description=Marketing site", gotBody)
	}
	if cfg := readProjectContextFile(t, path); cfg.Context.ProjectID != "p1" {
		t.Errorf("config = %+v, want project=p1", cfg.Context)
	}
}

func TestRunProjectCreate_RequiresWorkspace(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.WorkspaceID = "" })
	cmd := parsedCmd(t, "create", projectCreateFlags(), "Website")
	if err := runProjectCreate(context.Background(), cmd); err == nil {
		t.Error("runProjectCreate() error = nil, want a missing-workspace error")
	}
}

func TestRunProjectCreate_RequiresExactlyOneName(t *testing.T) {
	for _, args := range [][]string{{}, {"a", "b"}} {
		cmd := parsedCmd(t, "create", projectCreateFlags(), args...)
		if err := runProjectCreate(context.Background(), cmd); err == nil {
			t.Errorf("runProjectCreate(%v) error = nil, want usage error", args)
		}
	}
}

func TestProjectInput_AllOnboardingFields(t *testing.T) {
	cmd := parsedCmd(t, "create", projectCreateFlags(),
		"-d", "Marketing site",
		"--product-type", "b2b_saas",
		"--product-stage", "beta",
		"--target-audience", "PMs",
		"--target-audience", "Founders",
		"--excluded-audience", "Individual developers",
		"--business-goal", "Retention",
		"--business-goal", "Revenue",
		"Website",
	)
	got, err := projectInput(cmd)
	if err != nil {
		t.Fatalf("projectInput() error = %v", err)
	}
	want := api.ProjectInput{
		Name:             "Website",
		Description:      "Marketing site",
		ProductType:      "b2b_saas",
		ProductStage:     "beta",
		TargetAudience:   []string{"PMs", "Founders"},
		ExcludedAudience: []string{"Individual developers"},
		BusinessGoals:    []string{"Retention", "Revenue"},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("projectInput() = %+v, want %+v (business goals keep flag order = priority)", *got, want)
	}
}

func TestProjectInput_Validation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "minimal", args: []string{"Website"}},
		{name: "trims name", args: []string{"  Website  "}},
		{name: "empty name", args: []string{"   "}, wantErr: true},
		{name: "name too long", args: []string{strings.Repeat("a", 51)}, wantErr: true},
		{name: "name with slash", args: []string{"a/b"}, wantErr: true},
		{name: "name with angle bracket", args: []string{"a<b"}, wantErr: true},
		{name: "description too long", args: []string{"-d", strings.Repeat("a", 1001), "Website"}, wantErr: true},
		{name: "description at limit", args: []string{"-d", strings.Repeat("a", 1000), "Website"}},
		{name: "bad product type", args: []string{"--product-type", "saas", "Website"}, wantErr: true},
		{name: "bad product stage", args: []string{"--product-stage", "launched", "Website"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parsedCmd(t, "create", projectCreateFlags(), tt.args...)
			got, err := projectInput(cmd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("projectInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got.Name != "Website" {
				t.Errorf("projectInput().Name = %q, want %q", got.Name, "Website")
			}
		})
	}
}

func TestOneOf_AcceptsEveryDocumentedOption(t *testing.T) {
	for _, value := range append(append([]string{}, productTypes...), productStages...) {
		allowed := productTypes
		if slices.Contains(productStages, value) {
			allowed = productStages
		}
		if got, err := oneOf(value, allowed, "flag"); err != nil || got != value {
			t.Errorf("oneOf(%q) = (%q, %v), want (%q, nil)", value, got, err, value)
		}
	}
	if _, err := oneOf("", productTypes, "product-type"); err != nil {
		t.Errorf("oneOf(\"\") error = %v, want nil (the field is optional)", err)
	}
}
