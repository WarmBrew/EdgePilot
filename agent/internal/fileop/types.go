package fileop

import "time"

type FileInfo struct {
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Size          int64     `json:"size"`
	Mode          string    `json:"mode"`
	ModifiedAt    time.Time `json:"modified_at"`
	IsSymlink     bool      `json:"is_symlink"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
	Owner         string    `json:"owner"`
	Group         string    `json:"group"`
}

type ListDirRequest struct {
	Path string `json:"path"`
}

type ListDirResponse struct {
	Files []*FileInfo `json:"files"`
	Path  string      `json:"path"`
}

type ReadFileRequest struct {
	Path string `json:"path"`
}

type ReadFileResponse struct {
	Content  string `json:"content"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
	IsBinary bool   `json:"is_binary"`
}

type WriteFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode"`
}

type WriteFileResponse struct {
	Success      bool `json:"success"`
	BytesWritten int  `json:"bytes_written"`
}

type DeleteFileRequest struct {
	Path string `json:"path"`
}

type GetFileInfoRequest struct {
	Path string `json:"path"`
}

type ChmodRequest struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type ChownRequest struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
	Group string `json:"group"`
}
