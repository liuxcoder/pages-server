package gitea

type FileResponse struct {
	Exists   bool
	ETag     []byte
	MimeType string
	Body     []byte
}

func (f FileResponse) IsEmpty() bool {
	return len(f.Body) != 0
}
