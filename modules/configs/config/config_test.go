/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package config

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateVersion(t *testing.T) {
	input := `PEFFIX DON"T CHANGE,,, #MANAGED_CONFIG_VERSION: 1234, keep this as well`
	output := updateConfigVersion(input, 1235)
	fmt.Println(output)
	assert.Equal(t, output, `PEFFIX DON"T CHANGE,,, #MANAGED_CONFIG_VERSION: 1235, keep this as well`)

	input = `PEFFIX DON"T CHANGE, this is my config`
	output = updateConfigVersion(input, 1235)
	fmt.Println("new config:")
	fmt.Println(output)

	assert.Equal(t, output, "PEFFIX DON\"T CHANGE, this is my config\n#MANAGED_CONFIG_VERSION: 1235")
}

func TestVersion(t *testing.T) {
	ver := parseConfigVersion("#MANAGED_CONFIG_VERSION: 1234")
	assert.Equal(t, ver, int64(1234))

	ver = parseConfigVersion("#MANAGED_CONFIG_VERSION:1234")
	assert.Equal(t, ver, int64(1234))

	ver = parseConfigVersion("#MANAGED_CONFIG_VERSION:1234 ")
	assert.Equal(t, ver, int64(1234))

	ver = parseConfigVersion("##MANAGED_CONFIG_VERSION: 1234")
	assert.Equal(t, ver, int64(1234))

	ver = parseConfigVersion("what's the version, i think is 1234")
	assert.Equal(t, ver, int64(-1))
}

func TestSaveConfigStr(t *testing.T) {
	cfgDir, err := filepath.Abs(global.Env().GetConfigDir())
	if err != nil {
		t.Fatalf("Failed to get config directory: %v", err)
	}

	testFileName := "test_config.yaml"
	testContent := "test content"

	err = SaveConfigStr(testFileName, testContent)
	if err != nil {
		t.Fatalf("SaveConfigStr failed: %v", err)
	}

	savedFilePath := filepath.Join(cfgDir, testFileName)
	if !util.FileExists(savedFilePath) {
		t.Fatalf("Expected file %s to exist", savedFilePath)
	}

	savedContent, err := util.FileGetContent(savedFilePath)
	if err != nil {
		t.Fatalf("Failed to read saved file content: %v", err)
	}

	assert.Equal(t, string(savedContent), testContent)

	// Clean up
	err = os.Remove(savedFilePath)
	if err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}
}

// TestStripManagedMarkersSymmetry — the hash computed after the client
// strips the marker lines must equal the server-side one (raw lines,
// TrimSpace'd); otherwise every sync cycle would see a content change and
// rewrite the file.
func TestStripManagedMarkersSymmetry(t *testing.T) {
	row := "{\"pipeline\":[{\"name\":\"x\"}]}"
	stamped := row + "\n#MANAGED_CONFIG_VERSION: 15\n#MANAGED: true"
	if got := stripManagedMarkers(stamped); got != row {
		t.Fatalf("strip = %q, want %q", got, row)
	}
	// marker glued onto the row (historic stacking): the whole line goes
	glued := row + "\n#MANAGED: trueMANAGED_CONFIG_VERSION: 15"
	if got := stripManagedMarkers(glued); got != row {
		t.Fatalf("strip glued = %q, want %q", got, row)
	}
	if stripManagedMarkers(row) != row {
		t.Fatal("bare content must pass through")
	}
}
