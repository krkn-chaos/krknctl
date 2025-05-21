package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math"
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

func TestRandomInt64(t *testing.T) {
	randOne := RandomInt64(nil)
	randTwo := RandomInt64(nil)
	fmt.Printf("RandomInt64: %v, RandomInt64: %v\n", randOne, randTwo)
	assert.NotEqual(t, randOne, randTwo)

	// ensures that respects the limit
	maxInt := int64(math.MaxInt)
	for i := 0; i < 1000; i++ {
		maxRandInt := RandomInt64(&maxInt)
		assert.Greater(t, int64(math.MaxInt64), maxRandInt)
	}

}
