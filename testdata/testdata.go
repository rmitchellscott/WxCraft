package testdata

import (
	"bufio"
	"compress/gzip"
	"embed"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed *.gz
var data embed.FS

func newScanner(t *testing.T, path string) *bufio.Scanner {
	f, err := data.Open(path)
	require.NoError(t, err)

	r, err := gzip.NewReader(f)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, r.Close())
	})

	scanner := bufio.NewScanner(r)
	t.Cleanup(func() {
		require.NoError(t, scanner.Err())
	})

	return scanner
}

func METAR(t *testing.T) *bufio.Scanner {
	return newScanner(t, "metar.txt.gz")
}

func TAF(t *testing.T) *bufio.Scanner {
	return newScanner(t, "taf.txt.gz")
}
