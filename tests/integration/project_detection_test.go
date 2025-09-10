package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarkank/aimem/internal/project"
	"github.com/tarkank/aimem/internal/types"
)

// TestProjectDetectionScenarios tests various project detection scenarios
func TestProjectDetectionScenarios(t *testing.T) {
	detector := project.NewProjectDetector()

	t.Run("GitRepositoryDetection", func(t *testing.T) {
		testGitRepositoryDetection(t, detector)
	})

	t.Run("NodeJSProjectDetection", func(t *testing.T) {
		testNodeJSProjectDetection(t, detector)
	})

	t.Run("GoProjectDetection", func(t *testing.T) {
		testGoProjectDetection(t, detector)
	})

	t.Run("PythonProjectDetection", func(t *testing.T) {
		testPythonProjectDetection(t, detector)
	})

	t.Run("MultiLanguageProjectDetection", func(t *testing.T) {
		testMultiLanguageProjectDetection(t, detector)
	})

	t.Run("DirectoryProjectDetection", func(t *testing.T) {
		testDirectoryProjectDetection(t, detector)
	})

	t.Run("ProjectDetectionCaching", func(t *testing.T) {
		testProjectDetectionCaching(t, detector)
	})
}

func testGitRepositoryDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "git-project")

	// Create Git repository structure
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	// Create git config with remote
	gitConfig := `[core]
    repositoryformatversion = 0
    filemode = true
[remote "origin"]
    url = https://github.com/user/repo.git
    fetch = +refs/heads/*:refs/remotes/origin/*`

	err = os.WriteFile(filepath.Join(projectDir, ".git", "config"), []byte(gitConfig), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeGitRepository, projectInfo.Type)
	assert.Equal(t, "git-project", projectInfo.Name)
	assert.NotNil(t, projectInfo.GitRoot)
	assert.Equal(t, projectDir, *projectInfo.GitRoot)
	assert.NotNil(t, projectInfo.GitRemote)
	assert.Equal(t, "https://github.com/user/repo.git", *projectInfo.GitRemote)
	assert.NotEmpty(t, projectInfo.ID)
}

func testNodeJSProjectDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "node-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create package.json
	packageJSON := `{
  "name": "my-node-app",
  "version": "1.0.0",
  "description": "Test Node.js application",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "jest": "^28.0.0"
  }
}`

	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Create some JS files
	err = os.WriteFile(filepath.Join(projectDir, "index.js"), []byte("console.log('Hello World');"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectDir, "app.js"), []byte("const express = require('express');"), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeWorkspace, projectInfo.Type)
	assert.Equal(t, "node-project", projectInfo.Name)
	assert.Contains(t, projectInfo.WorkspaceMarkers, "package.json")
	assert.Equal(t, "JavaScript", projectInfo.Language)
}

func testGoProjectDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "go-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create go.mod
	goMod := `module github.com/user/go-project

go 1.19

require (
    github.com/gorilla/mux v1.8.0
    github.com/stretchr/testify v1.8.0
)`

	err = os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(t, err)

	// Create some Go files
	mainGo := `package main

import (
    "fmt"
    "github.com/gorilla/mux"
)

func main() {
    router := mux.NewRouter()
    fmt.Println("Server starting...")
}`

	err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainGo), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeWorkspace, projectInfo.Type)
	assert.Equal(t, "go-project", projectInfo.Name)
	assert.Contains(t, projectInfo.WorkspaceMarkers, "go.mod")
	assert.Equal(t, "Go", projectInfo.Language)
}

func testPythonProjectDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "python-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create requirements.txt
	requirements := `flask==2.2.0
requests==2.28.0
pytest==7.1.0
black==22.3.0`

	err = os.WriteFile(filepath.Join(projectDir, "requirements.txt"), []byte(requirements), 0644)
	require.NoError(t, err)

	// Create Python files
	appPy := `from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello_world():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(debug=True)`

	err = os.WriteFile(filepath.Join(projectDir, "app.py"), []byte(appPy), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeWorkspace, projectInfo.Type)
	assert.Equal(t, "python-project", projectInfo.Name)
	assert.Contains(t, projectInfo.WorkspaceMarkers, "requirements.txt")
	assert.Equal(t, "Python", projectInfo.Language)
}

func testMultiLanguageProjectDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "multi-lang-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create multiple workspace markers
	packageJSON := `{"name": "frontend", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	goMod := `module backend
go 1.19`
	err = os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(t, err)

	// Create files in both languages
	err = os.WriteFile(filepath.Join(projectDir, "frontend.js"), []byte("console.log('frontend');"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectDir, "backend.go"), []byte("package main\nfunc main() {}"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeWorkspace, projectInfo.Type)
	assert.Equal(t, "multi-lang-project", projectInfo.Name)
	assert.Contains(t, projectInfo.WorkspaceMarkers, "package.json")
	assert.Contains(t, projectInfo.WorkspaceMarkers, "go.mod")
	// Should detect Go as primary language (more .go files)
	assert.Equal(t, "Go", projectInfo.Language)
}

func testDirectoryProjectDetection(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "simple-directory")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create some random files without workspace markers
	err = os.WriteFile(filepath.Join(projectDir, "readme.txt"), []byte("Simple project"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectDir, "notes.md"), []byte("# Notes"), 0644)
	require.NoError(t, err)

	// Test detection
	projectInfo, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	assert.Equal(t, types.ProjectTypeDirectory, projectInfo.Type)
	assert.Equal(t, "simple-directory", projectInfo.Name)
	assert.Empty(t, projectInfo.WorkspaceMarkers)
	assert.Empty(t, projectInfo.Language)
	assert.Empty(t, projectInfo.Framework)
	assert.NotEmpty(t, projectInfo.ID)
}

func testProjectDetectionCaching(t *testing.T, detector *project.ProjectDetector) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "cached-project")
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	// First detection - should cache result
	projectInfo1, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	// Second detection - should use cached result
	projectInfo2, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	// Results should be identical
	assert.Equal(t, projectInfo1.ID, projectInfo2.ID)
	assert.Equal(t, projectInfo1.Type, projectInfo2.Type)
	assert.Equal(t, projectInfo1.Name, projectInfo2.Name)
	assert.Equal(t, projectInfo1.CanonicalPath, projectInfo2.CanonicalPath)

	// Clear cache and test again
	detector.ClearCache()

	projectInfo3, err := detector.DetectProject(projectDir)
	require.NoError(t, err)

	// Should still be same project but freshly detected
	assert.Equal(t, projectInfo1.ID, projectInfo3.ID)
	assert.Equal(t, projectInfo1.Type, projectInfo3.Type)
}

// TestFrameworkDetection tests specific framework detection
func TestFrameworkDetection(t *testing.T) {
	detector := project.NewProjectDetector()

	testCases := []struct {
		name              string
		files             map[string]string
		expectedFramework string
	}{
		{
			name: "NextJS",
			files: map[string]string{
				"package.json":   `{"name": "next-app", "dependencies": {"next": "^12.0.0"}}`,
				"next.config.js": "module.exports = {};",
			},
			expectedFramework: "Next.js",
		},
		{
			name: "Vue",
			files: map[string]string{
				"package.json":  `{"name": "vue-app", "dependencies": {"vue": "^3.0.0"}}`,
				"vue.config.js": "module.exports = {};",
			},
			expectedFramework: "Vue.js",
		},
		{
			name: "Angular",
			files: map[string]string{
				"package.json": `{"name": "angular-app", "dependencies": {"@angular/core": "^14.0.0"}}`,
				"angular.json": "{}",
			},
			expectedFramework: "Angular",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			projectDir := filepath.Join(tempDir, "framework-test")
			err := os.MkdirAll(projectDir, 0755)
			require.NoError(t, err)

			// Create test files
			for filename, content := range tc.files {
				err = os.WriteFile(filepath.Join(projectDir, filename), []byte(content), 0644)
				require.NoError(t, err)
			}

			// Test detection
			projectInfo, err := detector.DetectProject(projectDir)
			require.NoError(t, err)

			assert.Contains(t, projectInfo.Framework, tc.expectedFramework)
		})
	}
}

// BenchmarkProjectDetection benchmarks project detection performance
func BenchmarkProjectDetection(b *testing.B) {
	detector := project.NewProjectDetector()

	// Setup test project
	tempDir := b.TempDir()
	projectDir := filepath.Join(tempDir, "benchmark-project")
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	if err != nil {
		b.Fatal(err)
	}

	packageJSON := `{"name": "benchmark-project", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		b.Fatal(err)
	}

	// Create multiple JS files for realistic scenario
	for i := 0; i < 10; i++ {
		filename := filepath.Join(projectDir, fmt.Sprintf("file%d.js", i))
		content := fmt.Sprintf("// File %d\nconsole.log('file %d');", i, i)
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Benchmark detection
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.DetectProject(projectDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}
