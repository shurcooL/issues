package fs

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"

	"src.sourcegraph.com/vfs"
)

// fileInfoID describes a file, whose name is an ID of type uint64.
type fileInfoID struct {
	os.FileInfo
	ID uint64
}

// byID implements sort.Interface.
type byID []fileInfoID

func (f byID) Len() int           { return len(f) }
func (f byID) Less(i, j int) bool { return f[i].ID < f[j].ID }
func (f byID) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// readDirIDs reads the directory named by path and returns
// a list of directory entries whose names are IDs of type uint64, sorted by ID.
// Other entries with names don't match the naming scheme are ignored.
// If the directory doesn't exist, empty list and no error are returned.
func readDirIDs(fs vfs.FileSystem, path string) ([]fileInfoID, error) {
	fis, err := fs.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) { // Non-existing dirs are not considered an error.
			return nil, nil
		}
		return nil, err
	}
	var fiis []fileInfoID
	for _, fi := range fis {
		id, err := strconv.ParseUint(fi.Name(), 10, 64)
		if err != nil {
			continue
		}
		fiis = append(fiis, fileInfoID{
			FileInfo: fi,
			ID:       id,
		})
	}
	sort.Sort(byID(fiis))
	return fiis, nil
}

// jsonEncodeFile encodes v into file at path, overwriting or creating it.
func jsonEncodeFile(fs vfs.FileSystem, path string, v interface{}) error {
	f, err := fs.Create(path)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(v)
	_ = f.Close()
	if err != nil {
		return err
	}
	return nil
}

// jsonDecodeFile decodes contents of file at path into v.
func jsonDecodeFile(fs vfs.FileSystem, path string, v interface{}) error {
	f, err := fs.Open(path)
	if err != nil {
		return err
	}
	err = json.NewDecoder(f).Decode(v)
	_ = f.Close()
	if err != nil {
		return err
	}
	return nil
}
