package auth

import (
	context "context"
	"encoding/csv"
	"os"
	"sort"
	"strings"
)

type fileRecord struct {
	Username     string
	PasswordHash string
	MountPoint   string
}

type fileHandler struct {
	db []fileRecord
}

func (h *fileHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (string, error) {
	str := string(mqtt.Username)
	idx := sort.Search(len(h.db), func(i int) bool { return h.db[i].Username == str })
	if idx < len(h.db) && h.db[idx].Username == str {
		return h.db[idx].MountPoint, nil
	}
	return "", ErrAuthenticationFailed
}

func searchHelper(out []fileRecord) func(i, j int) bool {
	return func(i, j int) bool {
		return strings.Compare(out[i].Username, out[j].Username) == -1
	}
}

// FileHandler returns a file-based authentication handler.
// It reads a csv file, where each lines are formated line "username:password:mountpoint".
// If mountpoint is empty, the default mountpoint will be used.
func FileHandler(path string) (AuthenticationHandler, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	reader := csv.NewReader(fd)
	reader.Comma = ':'
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	out := make([]fileRecord, 0)
	for idx := range records {
		switch len(records[idx]) {
		case 2:
			out = append(out, fileRecord{
				Username:     records[idx][0],
				PasswordHash: records[idx][1],
				MountPoint:   DefaultMountPoint,
			})
		case 3:
			out = append(out, fileRecord{
				Username:     records[idx][0],
				PasswordHash: records[idx][1],
				MountPoint:   records[idx][3],
			})
		}
	}
	sort.SliceStable(out, searchHelper(out))
	return &fileHandler{
		db: out,
	}, nil
}
