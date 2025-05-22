package goprotos7

type Options struct {
	// store the binary files folder
	BinFilesFolder string `json:"bin_files_folder"`
}

type ServerOption func(*Options)

func WithBinFilesFolder(binFilesFolder string) ServerOption {
	return func(o *Options) {
		o.BinFilesFolder = binFilesFolder
	}
}
