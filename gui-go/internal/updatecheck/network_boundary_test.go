package updatecheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestOutboundNetworkingIsIsolatedToTheOptInUpdater guards the app's network
// boundary: direct HTTP/socket/subprocess imports stay in updatecheck, Fyne's
// remote resource loaders are unused, update calls stay behind consent, and
// the updater has no init hook.
func TestOutboundNetworkingIsIsolatedToTheOptInUpdater(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	sourceRoot := filepath.Join(moduleRoot, "gui-go")
	const updaterDir = "internal/updatecheck"
	const updateFlowFile = "internal/ui/updates.go"

	err := filepath.WalkDir(sourceRoot, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		relative, err := filepath.Rel(sourceRoot, filename)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			return err
		}
		inUpdater := relative == updaterDir || strings.HasPrefix(relative, updaterDir+"/")
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if !inUpdater && isDirectNetworkImport(importPath) {
				t.Errorf("%s imports network-capable package %q outside the opt-in updater", relative, importPath)
			}
			if importPath == "github.com/Z3r0s/Pulse-Vault/gui-go/internal/updatecheck" && relative != updateFlowFile {
				t.Errorf("%s imports the updater outside the explicit update flow", relative)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if inUpdater {
				if function, ok := node.(*ast.FuncDecl); ok && function.Name.Name == "init" {
					t.Errorf("%s declares init; updater network work must not start during application launch", relative)
				}
			}
			return true
		})
		auditUpdateCallSites(t, relative, updateFlowFile, file)
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources for network clients: %v", err)
	}
}

// auditUpdateCallSites keeps even the updater's network methods behind the
// explicit consent callback and the second, user-selected download action.
func auditUpdateCallSites(t *testing.T, relative, updateFlowFile string, file *ast.File) {
	t.Helper()
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		functionName := function.Name.Name
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selector, isSelector := node.(*ast.SelectorExpr)
			if isSelector && selector.Sel.Name == "updateClient" {
				if relative != updateFlowFile || (functionName != "checkForUpdates" && functionName != "downloadUpdate") {
					t.Errorf("%s accesses the updater client outside an explicit update action (%s)", relative, functionName)
				}
			}

			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			called, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch called.Sel.Name {
			case "LoadResourceFromURLString", "CacheResourceFromURLString", "LoadResourceFromURI":
				t.Errorf("%s calls Fyne's network-capable resource loader (%s); remote loads must not bypass update consent", relative, called.Sel.Name)
			}

			if relative == updateFlowFile {
				if packageName, ok := called.X.(*ast.Ident); ok && packageName.Name == "updatecheck" && called.Sel.Name == "New" {
					if functionName != "checkForUpdates" && functionName != "downloadUpdate" {
						t.Errorf("%s initializes the updater outside an explicit update action", functionName)
					}
				}
			}

			switch called.Sel.Name {
			case "Check":
				if relative != updateFlowFile || functionName != "checkForUpdates" {
					t.Errorf("%s calls the update check outside the explicit check action (%s)", relative, functionName)
				}
			case "Download":
				if relative != updateFlowFile || functionName != "downloadUpdate" {
					t.Errorf("%s downloads an update outside the explicit download action (%s)", relative, functionName)
				}
			}

			switch called.Sel.Name {
			case "checkForUpdates":
				if relative != updateFlowFile || functionName != "showUpdateConsent" {
					t.Errorf("%s can start an update check outside the consent dialog (%s)", relative, functionName)
				}
			case "downloadUpdate":
				if relative != updateFlowFile || functionName != "chooseUpdateDestination" {
					t.Errorf("%s can start an update download outside destination selection (%s)", relative, functionName)
				}
			case "showUpdateConsent":
				if relative != updateFlowFile || (functionName != "promptForUpdateAtStartup" && functionName != "onCheckUpdates") {
					t.Errorf("%s opens update consent from an unexpected code path (%s)", relative, functionName)
				}
			}
			return true
		})
	}
}

func isDirectNetworkImport(importPath string) bool {
	if importPath == "net" || importPath == "net/http" || importPath == "net/rpc" || importPath == "net/smtp" || importPath == "os/exec" {
		return true
	}
	for _, prefix := range []string{
		"net/http/",
		"golang.org/x/net/",
		"google.golang.org/grpc",
		"github.com/gorilla/websocket",
		"github.com/coder/websocket",
		"nhooyr.io/websocket",
	} {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}
	return false
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("could not locate go.mod for network boundary audit")
		}
		current = parent
	}
}
