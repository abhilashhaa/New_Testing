package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	piperhttp "github.com/SAP/jenkins-library/pkg/http"
	"github.com/SAP/jenkins-library/pkg/mock"

	"github.com/stretchr/testify/assert"
)

type detectTestUtilsBundle struct {
	expectedError   error
	downloadedFiles map[string]string // src, dest
	*mock.ShellMockRunner
	*mock.FilesMock
}

func (c *detectTestUtilsBundle) RunExecutable(string, ...string) error {
	panic("not expected to be called in test")
}

func (c *detectTestUtilsBundle) SetOptions(piperhttp.ClientOptions) {

}

func (c *detectTestUtilsBundle) DownloadFile(url, filename string, _ http.Header, _ []*http.Cookie) error {

	if c.expectedError != nil {
		return c.expectedError
	}

	if c.downloadedFiles == nil {
		c.downloadedFiles = make(map[string]string)
	}
	c.downloadedFiles[url] = filename
	return nil
}

func newDetectTestUtilsBundle() *detectTestUtilsBundle {
	utilsBundle := detectTestUtilsBundle{
		ShellMockRunner: &mock.ShellMockRunner{},
		FilesMock:       &mock.FilesMock{},
	}
	return &utilsBundle
}

func TestRunDetect(t *testing.T) {
	t.Parallel()
	t.Run("success case", func(t *testing.T) {
		t.Parallel()
		utilsMock := newDetectTestUtilsBundle()
		utilsMock.AddFile("detect.sh", []byte(""))
		err := runDetect(detectExecuteScanOptions{}, utilsMock)

		assert.Equal(t, utilsMock.downloadedFiles["https://detect.synopsys.com/detect.sh"], "detect.sh")
		assert.True(t, utilsMock.HasRemovedFile("detect.sh"))
		assert.NoError(t, err)
		assert.Equal(t, ".", utilsMock.Dir, "Wrong execution directory used")
		assert.Equal(t, "/bin/bash", utilsMock.Shell[0], "Bash shell expected")
		expectedScript := "./detect.sh --blackduck.url= --blackduck.api.token= \"--detect.project.name=''\" \"--detect.project.version.name=''\" \"--detect.code.location.name=''\" --detect.source.path='.'"
		assert.Equal(t, expectedScript, utilsMock.Calls[0])
	})

	t.Run("success case - with report", func(t *testing.T) {
		t.Parallel()
		utilsMock := newDetectTestUtilsBundle()
		utilsMock.AddFile("detect.sh", []byte(""))
		utilsMock.AddFile("my_BlackDuck_RiskReport.pdf", []byte(""))
		err := runDetect(detectExecuteScanOptions{FailOn: []string{"BLOCKER"}}, utilsMock)

		assert.Equal(t, utilsMock.downloadedFiles["https://detect.synopsys.com/detect.sh"], "detect.sh")
		assert.True(t, utilsMock.HasRemovedFile("detect.sh"))
		assert.NoError(t, err)
		assert.Equal(t, ".", utilsMock.Dir, "Wrong execution directory used")
		assert.Equal(t, "/bin/bash", utilsMock.Shell[0], "Bash shell expected")
		expectedScript := "./detect.sh --blackduck.url= --blackduck.api.token= \"--detect.project.name=''\" \"--detect.project.version.name=''\" --detect.policy.check.fail.on.severities=BLOCKER \"--detect.code.location.name=''\" --detect.source.path='.'"
		assert.Equal(t, expectedScript, utilsMock.Calls[0])

		content, err := utilsMock.FileRead("blackduck-ip.json")
		assert.NoError(t, err)
		assert.Contains(t, string(content), `"policyViolations":0`)
	})

	t.Run("failure case", func(t *testing.T) {
		t.Parallel()
		utilsMock := newDetectTestUtilsBundle()
		utilsMock.ShouldFailOnCommand = map[string]error{"./detect.sh --blackduck.url= --blackduck.api.token= \"--detect.project.name=''\" \"--detect.project.version.name=''\" \"--detect.code.location.name=''\" --detect.source.path='.'": fmt.Errorf("Test Error")}
		utilsMock.AddFile("detect.sh", []byte(""))
		err := runDetect(detectExecuteScanOptions{}, utilsMock)
		assert.EqualError(t, err, "Test Error")
		assert.True(t, utilsMock.HasRemovedFile("detect.sh"))
	})

	t.Run("maven parameters", func(t *testing.T) {
		t.Parallel()
		utilsMock := newDetectTestUtilsBundle()
		utilsMock.CurrentDir = "root_folder"
		utilsMock.AddFile("detect.sh", []byte(""))
		err := runDetect(detectExecuteScanOptions{
			M2Path:              ".pipeline/local_repo",
			ProjectSettingsFile: "project-settings.xml",
			GlobalSettingsFile:  "global-settings.xml",
		}, utilsMock)

		assert.NoError(t, err)
		assert.Equal(t, ".", utilsMock.Dir, "Wrong execution directory used")
		assert.Equal(t, "/bin/bash", utilsMock.Shell[0], "Bash shell expected")
		absoluteLocalPath := string(os.PathSeparator) + filepath.Join("root_folder", ".pipeline", "local_repo")

		expectedParam := "\"--detect.maven.build.command='--global-settings global-settings.xml --settings project-settings.xml -Dmaven.repo.local=" + absoluteLocalPath + "'\""
		assert.Contains(t, utilsMock.Calls[0], expectedParam)
	})
}

func TestAddDetectArgs(t *testing.T) {
	t.Parallel()
	testData := []struct {
		args     []string
		options  detectExecuteScanOptions
		expected []string
	}{
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ScanProperties:  []string{"--scan1=1", "--scan2=2"},
				ServerURL:       "https://server.url",
				Token:           "apiToken",
				ProjectName:     "testName",
				Version:         "1.0",
				VersioningModel: "major-minor",
				CodeLocation:    "",
				Scanners:        []string{"signature"},
				ScanPaths:       []string{"path1", "path2"},
			},
			expected: []string{
				"--testProp1=1",
				"--scan1=1",
				"--scan2=2",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.code.location.name='testName/1.0'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path='.'",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:       "https://server.url",
				Token:           "apiToken",
				ProjectName:     "testName",
				Version:         "1.0",
				VersioningModel: "major-minor",
				CodeLocation:    "testLocation",
				FailOn:          []string{"BLOCKER", "MAJOR"},
				Scanners:        []string{"source"},
				ScanPaths:       []string{"path1", "path2"},
				Groups:          []string{"testGroup"},
			},
			expected: []string{
				"--testProp1=1",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path='.'",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:       "https://server.url",
				Token:           "apiToken",
				ProjectName:     "testName",
				CodeLocation:    "testLocation",
				FailOn:          []string{"BLOCKER", "MAJOR"},
				Scanners:        []string{"source"},
				ScanPaths:       []string{"path1", "path2"},
				Groups:          []string{"testGroup", "testGroup2"},
				Version:         "1.0",
				VersioningModel: "major-minor",
			},
			expected: []string{
				"--testProp1=1",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path='.'",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:       "https://server.url",
				Token:           "apiToken",
				ProjectName:     "testName",
				CodeLocation:    "testLocation",
				FailOn:          []string{"BLOCKER", "MAJOR"},
				Scanners:        []string{"source"},
				ScanPaths:       []string{"path1", "path2"},
				Groups:          []string{"testGroup", "testGroup2"},
				Version:         "1.0",
				VersioningModel: "major-minor",
				DependencyPath:  "pathx",
			},
			expected: []string{
				"--testProp1=1",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:       "https://server.url",
				Token:           "apiToken",
				ProjectName:     "testName",
				CodeLocation:    "testLocation",
				FailOn:          []string{"BLOCKER", "MAJOR"},
				Scanners:        []string{"source"},
				ScanPaths:       []string{"path1", "path2"},
				Groups:          []string{"testGroup", "testGroup2"},
				Version:         "1.0",
				VersioningModel: "major-minor",
				DependencyPath:  "pathx",
				Unmap:           true,
			},
			expected: []string{
				"--testProp1=1",
				"--detect.project.codelocation.unmap=true",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:               "https://server.url",
				Token:                   "apiToken",
				ProjectName:             "testName",
				CodeLocation:            "testLocation",
				FailOn:                  []string{"BLOCKER", "MAJOR"},
				Scanners:                []string{"source"},
				ScanPaths:               []string{"path1", "path2"},
				Groups:                  []string{"testGroup", "testGroup2"},
				Version:                 "1.0",
				VersioningModel:         "major-minor",
				DependencyPath:          "pathx",
				Unmap:                   true,
				IncludedPackageManagers: []string{"maven", "GRADLE"},
				ExcludedPackageManagers: []string{"npm", "NUGET"},
				MavenExcludedScopes:     []string{"TEST", "compile"},
				DetectTools:             []string{"DETECTOR"},
			},
			expected: []string{
				"--testProp1=1",
				"--detect.project.codelocation.unmap=true",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
				"--detect.included.detector.types=MAVEN,GRADLE",
				"--detect.excluded.detector.types=NPM,NUGET",
				"--detect.maven.excluded.scopes=test,compile",
				"--detect.tools=DETECTOR",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:               "https://server.url",
				Token:                   "apiToken",
				ProjectName:             "testName",
				CodeLocation:            "testLocation",
				FailOn:                  []string{"BLOCKER", "MAJOR"},
				Scanners:                []string{"source"},
				ScanPaths:               []string{"path1", "path2"},
				Groups:                  []string{"testGroup", "testGroup2"},
				Version:                 "1.0",
				VersioningModel:         "major-minor",
				DependencyPath:          "pathx",
				Unmap:                   true,
				IncludedPackageManagers: []string{"maven", "GRADLE"},
				ExcludedPackageManagers: []string{"npm", "NUGET"},
				MavenExcludedScopes:     []string{"TEST", "compile"},
				DetectTools:             []string{"DETECTOR"},
				ScanOnChanges:           true,
			},
			expected: []string{
				"--testProp1=1",
				"--report",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
				"--detect.included.detector.types=MAVEN,GRADLE",
				"--detect.excluded.detector.types=NPM,NUGET",
				"--detect.maven.excluded.scopes=test,compile",
				"--detect.tools=DETECTOR",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ServerURL:               "https://server.url",
				Token:                   "apiToken",
				ProjectName:             "testName",
				CodeLocation:            "testLocation",
				FailOn:                  []string{"BLOCKER", "MAJOR"},
				Scanners:                []string{"source"},
				ScanPaths:               []string{"path1", "path2"},
				Groups:                  []string{"testGroup", "testGroup2"},
				Version:                 "1.0",
				VersioningModel:         "major-minor",
				DependencyPath:          "pathx",
				Unmap:                   true,
				IncludedPackageManagers: []string{"maven", "GRADLE"},
				ExcludedPackageManagers: []string{"npm", "NUGET"},
				MavenExcludedScopes:     []string{"TEST", "compile"},
				DetectTools:             []string{"DETECTOR"},
				ScanOnChanges:           true,
			},
			expected: []string{
				"--testProp1=1",
				"--report",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
				"--detect.included.detector.types=MAVEN,GRADLE",
				"--detect.excluded.detector.types=NPM,NUGET",
				"--detect.maven.excluded.scopes=test,compile",
				"--detect.tools=DETECTOR",
			},
		},
		{
			args: []string{"--testProp1=1"},
			options: detectExecuteScanOptions{
				ScanProperties:          []string{"--scan=1", "--detect.project.codelocation.unmap=true"},
				ServerURL:               "https://server.url",
				Token:                   "apiToken",
				ProjectName:             "testName",
				CodeLocation:            "testLocation",
				FailOn:                  []string{"BLOCKER", "MAJOR"},
				Scanners:                []string{"source"},
				ScanPaths:               []string{"path1", "path2"},
				Groups:                  []string{"testGroup", "testGroup2"},
				Version:                 "1.0",
				VersioningModel:         "major-minor",
				DependencyPath:          "pathx",
				Unmap:                   true,
				IncludedPackageManagers: []string{"maven", "GRADLE"},
				ExcludedPackageManagers: []string{"npm", "NUGET"},
				MavenExcludedScopes:     []string{"TEST", "compile"},
				DetectTools:             []string{"DETECTOR"},
				ScanOnChanges:           true,
			},
			expected: []string{
				"--testProp1=1",
				"--report",
				"--scan=1",
				"--blackduck.url=https://server.url",
				"--blackduck.api.token=apiToken",
				"\"--detect.project.name='testName'\"",
				"\"--detect.project.version.name='1.0'\"",
				"\"--detect.project.user.groups='testGroup,testGroup2'\"",
				"--detect.policy.check.fail.on.severities=BLOCKER,MAJOR",
				"\"--detect.code.location.name='testLocation'\"",
				"--detect.blackduck.signature.scanner.paths=path1,path2",
				"--detect.source.path=pathx",
				"--detect.included.detector.types=MAVEN,GRADLE",
				"--detect.excluded.detector.types=NPM,NUGET",
				"--detect.maven.excluded.scopes=test,compile",
				"--detect.tools=DETECTOR",
			},
		},
	}

	for k, v := range testData {
		v := v
		t.Run(fmt.Sprintf("run %v", k), func(t *testing.T) {
			t.Parallel()
			got, err := addDetectArgs(v.args, v.options, newDetectTestUtilsBundle())
			assert.NoError(t, err)
			assert.Equal(t, v.expected, got)
		})
	}
}
