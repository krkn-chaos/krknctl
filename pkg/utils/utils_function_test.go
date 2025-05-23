package utils

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandFolder(t *testing.T) {

	currentfolder, err := os.Getwd()
	parentfolder := filepath.Dir(currentfolder)
	grandParentFolder := filepath.Dir(parentfolder)
	baseFolder := "/usr/local/bin"
	homeFolder, err := os.UserHomeDir()
	assert.Nil(t, err)
	expectedHomeFolder := filepath.Join(homeFolder, "tests", "data")
	expectedParentFolder := filepath.Join(parentfolder, "krknctl")
	expectedGrandParentFolder := filepath.Join(grandParentFolder, "krknctl")
	expectedCurrentFolder := filepath.Join(currentfolder, "krknctl")
	expectedParentFolderWithBase := "/usr/local/krknctl"

	resultParentFolder, err := ExpandFolder("../krknctl", nil)
	assert.Nil(t, err)
	assert.Equal(t, expectedParentFolder, *resultParentFolder)

	resultGrandParentFolder, err := ExpandFolder("../../krknctl", nil)
	assert.Nil(t, err)
	assert.Equal(t, expectedGrandParentFolder, *resultGrandParentFolder)

	resultCurrentFolder, err := ExpandFolder("./krknctl", nil)
	assert.Nil(t, err)
	assert.Equal(t, expectedCurrentFolder, *resultCurrentFolder)

	resultHomeFolder, err := ExpandFolder("~/tests/data", nil)
	assert.Nil(t, err)
	assert.Equal(t, expectedHomeFolder, *resultHomeFolder)

	resultSameAbsoluteFolder, err := ExpandFolder(currentfolder, nil)
	assert.Nil(t, err)
	assert.Equal(t, currentfolder, *resultSameAbsoluteFolder)

	resultParentFolderWithBase, err := ExpandFolder("../krknctl", &baseFolder)
	assert.Nil(t, err)
	assert.Equal(t, expectedParentFolderWithBase, *resultParentFolderWithBase)

}
