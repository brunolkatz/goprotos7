package goprotos7

type Options struct {
	// store the binary files folder
	BinFilesFolder string `json:"bin_files_folder"`

	Transport *Transport `json:"transport"` // Transport layer (TCP/UDP) definition
}

type ServerOption func(*Options)

func WithBinFilesFolder(binFilesFolder string) ServerOption {
	return func(o *Options) {
		o.BinFilesFolder = binFilesFolder
	}
}

func WithPort(port int) ServerOption {
	return func(o *Options) {
		o.Transport.Port = port
	}
}

func WithTransport() ServerOption {
	return func(o *Options) {
		o.Transport.Local = true
	}
}
