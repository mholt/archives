package archives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestTrimTopDir(t *testing.T) {
	for i, test := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "b/c"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "def"},
		{input: "/abc/def", want: "def"},
	} {
		t.Run(test.input, func(t *testing.T) {
			got := trimTopDir(test.input)
			if got != test.want {
				t.Errorf("Test %d: want: '%s', got: '%s')", i, test.want, got)
			}
		})
	}
}

func TestTopDir(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "a"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "abc"},
		{input: "/abc/def", want: "/abc"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := topDir(tc.input)
			if got != tc.want {
				t.Errorf("want: '%s', got: '%s')", tc.want, got)
			}
		})
	}
}

func TestFileIsIncluded(t *testing.T) {
	for i, tc := range []struct {
		included  []string
		candidate string
		expect    bool
	}{
		{
			included:  []string{"a"},
			candidate: "a",
			expect:    true,
		},
		{
			included:  []string{"a", "b", "a/b"},
			candidate: "b",
			expect:    true,
		},
		{
			included:  []string{"a", "b", "c/d"},
			candidate: "c/d/e",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "a/b/c",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "aa/b/c",
			expect:    false,
		},
		{
			included:  []string{"a", "b", "c/d"},
			candidate: "b/c",
			expect:    true,
		},
		{
			included:  []string{"a/"},
			candidate: "a",
			expect:    false,
		},
		{
			included:  []string{"a/"},
			candidate: "a/",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "a/",
			expect:    true,
		},
		{
			included:  []string{"a/b"},
			candidate: "a/",
			expect:    false,
		},
	} {
		actual := fileIsIncluded(tc.included, tc.candidate)
		if actual != tc.expect {
			t.Errorf("Test %d (included=%v candidate=%v): expected %t but got %t",
				i, tc.included, tc.candidate, tc.expect, actual)
		}
	}
}

func TestSkipList(t *testing.T) {
	for i, tc := range []struct {
		start  skipList
		add    string
		expect skipList
	}{
		{
			start:  skipList{"a", "b", "c"},
			add:    "d",
			expect: skipList{"a", "b", "c", "d"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b",
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c", // don't add because b implies b/c
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c/", // effectively same as above
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b/", "c"},
			add:    "b", // effectively same as b/
			expect: skipList{"a", "b/", "c"},
		},
		{
			start:  skipList{"a", "b/c", "c"},
			add:    "b", // replace b/c because b is broader
			expect: skipList{"a", "c", "b"},
		},
	} {
		start := make(skipList, len(tc.start))
		copy(start, tc.start)

		tc.start.add(tc.add)

		if !reflect.DeepEqual(tc.start, tc.expect) {
			t.Errorf("Test %d (start=%v add=%v): expected %v but got %v",
				i, start, tc.add, tc.expect, tc.start)
		}
	}
}

func TestNameOnDiskToNameInArchive(t *testing.T) {
	for i, tc := range []struct {
		windows       bool   // only run this test on Windows
		rootOnDisk    string // user says they want to archive this file/folder
		nameOnDisk    string // the walk encounters a file with this name (with rootOnDisk as a prefix)
		rootInArchive string // file should be placed in this dir within the archive (rootInArchive becomes a prefix)
		expect        string // final filename in archive
	}{
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "a/b/c",
		},
		{
			rootOnDisk:    "a/b",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "b/c",
		},
		{
			rootOnDisk:    "a/b/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b/",
			nameOnDisk:    "a/b/c",
			rootInArchive: ".",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b/c",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/c",
		},
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo/",
			expect:        "foo/a/b/c",
		},
		{
			rootOnDisk:    "a/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			rootOnDisk:    "a/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			windows:       true,
			rootOnDisk:    `C:\foo`,
			nameOnDisk:    `C:\foo\bar`,
			rootInArchive: "",
			expect:        "foo/bar",
		},
		{
			windows:       true,
			rootOnDisk:    `C:\foo`,
			nameOnDisk:    `C:\foo\bar`,
			rootInArchive: "subfolder",
			expect:        "subfolder/bar",
		},
	} {
		if !strings.HasPrefix(tc.nameOnDisk, tc.rootOnDisk) {
			t.Errorf("Test %d: Invalid test case! Filename (on disk) will have rootOnDisk as a prefix according to the fs.WalkDirFunc godoc.", i)
			continue
		}
		if tc.windows && runtime.GOOS != "windows" {
			t.Logf("Test %d: Skipping test that is only compatible with Windows", i)
			continue
		}
		if !tc.windows && runtime.GOOS == "windows" {
			t.Logf("Test %d: Skipping test that is not compatible with Windows", i)
			continue
		}

		actual := nameOnDiskToNameInArchive(tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
		if actual != tc.expect {
			t.Errorf("Test %d: Got '%s' but expected '%s' (nameOnDisk=%s rootOnDisk=%s rootInArchive=%s)",
				i, actual, tc.expect, tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
		}
	}
}

func fixSeparators(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}

func TestFollowSymlink(t *testing.T) {
	// Create temp directory for tests
	tmpDir := t.TempDir()

	fixSeparators := func(path string) string {
		if runtime.GOOS == "windows" {
			return strings.ReplaceAll(path, "/", "\\")
		}
		return path
	}

	t.Run("single symlink to regular file", func(t *testing.T) {
		// Create a regular file
		targetFile := filepath.Join(tmpDir, "target.txt")
		if err := os.WriteFile(targetFile, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create symlink to the file
		symlinkFile := filepath.Join(tmpDir, "link.txt")
		if err := os.Symlink(targetFile, symlinkFile); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink
		finalPath, info, err := followSymlink(symlinkFile)
		if err != nil {
			t.Fatalf("followSymlink failed: %v", err)
		}

		if finalPath != fixSeparators(targetFile) {
			t.Errorf("expected final path %s, got %s", fixSeparators(targetFile), finalPath)
		}

		if info.IsDir() {
			t.Error("expected file, got directory")
		}

		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("expected regular file, got symlink")
		}
	})

	t.Run("chain of symlinks", func(t *testing.T) {
		// Create a regular file
		targetFile := filepath.Join(tmpDir, "chain_target.txt")
		if err := os.WriteFile(targetFile, []byte("chain content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create first symlink pointing to the file
		link1 := filepath.Join(tmpDir, "chain_link1.txt")
		if err := os.Symlink(targetFile, link1); err != nil {
			t.Fatal(err)
		}

		// Create second symlink pointing to first symlink
		link2 := filepath.Join(tmpDir, "chain_link2.txt")
		if err := os.Symlink(link1, link2); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink on the chain
		finalPath, info, err := followSymlink(link2)
		if err != nil {
			t.Fatalf("followSymlink failed: %v", err)
		}

		if finalPath != fixSeparators(targetFile) {
			t.Errorf("expected final path %s, got %s", fixSeparators(targetFile), finalPath)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("expected regular file, got symlink")
		}
	})

	t.Run("symlink loop detection", func(t *testing.T) {
		// Create circular symlinks
		loop1 := filepath.Join(tmpDir, "loop1.txt")
		loop2 := filepath.Join(tmpDir, "loop2.txt")

		// Create symlinks that point to each other
		if err := os.Symlink(loop2, loop1); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(loop1, loop2); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink should detect the loop
		_, _, err := followSymlink(loop1)
		if err == nil {
			t.Error("expected error for symlink loop, got nil")
		}
		if !strings.Contains(err.Error(), "symlink loop") {
			t.Errorf("expected 'symlink loop' error, got: %v", err)
		}
	})

	t.Run("relative path symlink", func(t *testing.T) {
		// Create subdirectory
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create target file in subdirectory
		targetFile := filepath.Join(tmpDir, "relative_target.txt")
		if err := os.WriteFile(targetFile, []byte("relative content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create symlink with relative path from tmpDir to subdir/target
		symlinkFile := filepath.Join(subDir, "relative_link.txt")
		if err := os.Symlink("../relative_target.txt", symlinkFile); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink
		finalPath, info, err := followSymlink(symlinkFile)
		if err != nil {
			t.Fatalf("followSymlink failed: %v", err)
		}

		if finalPath != fixSeparators(targetFile) {
			t.Errorf("expected final path %s, got %s", targetFile, finalPath)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("expected regular file, got symlink")
		}
	})

	t.Run("absolute path symlink", func(t *testing.T) {
		// Create target file
		targetFile := filepath.Join(tmpDir, "abs_target.txt")
		if err := os.WriteFile(targetFile, []byte("absolute content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create symlink with absolute path
		symlinkFile := filepath.Join(tmpDir, "abs_link.txt")
		if err := os.Symlink(targetFile, symlinkFile); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink
		finalPath, info, err := followSymlink(symlinkFile)
		if err != nil {
			t.Fatalf("followSymlink failed: %v", err)
		}

		if finalPath != fixSeparators(targetFile) {
			t.Errorf("expected final path %s, got %s", fixSeparators(targetFile), finalPath)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("expected regular file, got symlink")
		}
	})

	t.Run("broken symlink", func(t *testing.T) {
		// Create symlink pointing to non-existent file
		brokenLink := filepath.Join(tmpDir, "broken_link.txt")
		nonExistentTarget := filepath.Join(tmpDir, "nonexistent.txt")
		if err := os.Symlink(nonExistentTarget, brokenLink); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink should return error
		_, _, err := followSymlink(brokenLink)
		if err == nil {
			t.Error("expected error for broken symlink, got nil")
		}
		if !strings.Contains(err.Error(), "statting dereferenced symlink") {
			t.Errorf("expected 'statting dereferenced symlink' error, got: %v", err)
		}
	})

	t.Run("symlink to directory", func(t *testing.T) {
		// Create target directory
		targetDir := filepath.Join(tmpDir, "target_dir")
		if err := os.Mkdir(targetDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create symlink to directory
		symlinkDir := filepath.Join(tmpDir, "link_dir")
		if err := os.Symlink(targetDir, symlinkDir); err != nil {
			t.Fatal(err)
		}

		// Test followSymlink
		finalPath, info, err := followSymlink(symlinkDir)
		if err != nil {
			t.Fatalf("followSymlink failed: %v", err)
		}

		if finalPath != fixSeparators(targetDir) {
			t.Errorf("expected final path %s, got %s", fixSeparators(targetDir), finalPath)
		}

		if !info.IsDir() {
			t.Error("expected directory, got file")
		}

		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("expected regular directory, got symlink")
		}
	})

	t.Run("maximum symlink depth exceeded", func(t *testing.T) {
		// Create target file
		targetFile := filepath.Join(tmpDir, "depth_target.txt")
		if err := os.WriteFile(targetFile, []byte("depth content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a chain of 41 symlinks (exceeding the limit of 40)
		prevLink := targetFile
		var links []string
		for i := 0; i < 41; i++ {
			linkName := filepath.Join(tmpDir, fmt.Sprintf("depth_link_%d.txt", i))
			if err := os.Symlink(prevLink, linkName); err != nil {
				t.Fatal(err)
			}
			links = append(links, linkName)
			prevLink = linkName
		}

		// Test followSymlink should return depth error
		_, _, err := followSymlink(links[len(links)-1])
		if err == nil {
			t.Error("expected error for maximum depth exceeded, got nil")
		}
		if !strings.Contains(err.Error(), "maximum symlink depth") {
			t.Errorf("expected 'maximum symlink depth' error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "40") {
			t.Errorf("expected error to mention depth limit of 40, got: %v", err)
		}
	})
}

func TestFilesFromDisk_SymlinkOutsideFileNamesMap(t *testing.T) {
	tmpDir := t.TempDir()
	otherTmpDir := t.TempDir()

	testDirName := "test_dir"
	testDir := filepath.Join(otherTmpDir, testDirName)
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFileName := "test.txt"
	testFile := filepath.Join(testDir, testFileName)
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkDirName := "symlink_dir"
	symlinkDir := filepath.Join(tmpDir, symlinkDirName)
	if err := os.Symlink(testDir, symlinkDir); err != nil {
		t.Fatal(err)
	}

	files, err := FilesFromDisk(context.Background(), &FromDiskOptions{
		FollowSymlinks: true,
	}, map[string]string{symlinkDir: ""})
	if err != nil {
		t.Fatal(err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].NameInArchive < files[j].NameInArchive
	})

	if files[0].NameInArchive != symlinkDirName {
		t.Fatalf("expected file name '%s', got '%s'", symlinkDirName, files[0].NameInArchive)
	}

	testFilePath := fmt.Sprintf("%s/%s", symlinkDirName, testFileName)
	if files[1].NameInArchive != testFilePath {
		t.Fatalf("expected file name '%s', got '%s'", testFilePath, files[1].NameInArchive)
	}
}
