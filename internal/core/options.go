package core

// Options mirrors the upstream NSZ CLI/GUI flag surface.
type Options struct {
	Files []string

	Compress    bool
	CompressXCI bool // .xci → .xcz; off by default (experimental)
	Decompress  bool
	Info        bool
	Extract     bool
	Titlekeys   bool
	Verify      bool
	QuickVerify bool
	Keep        bool
	FixPadding  bool
	Long        bool
	Block       bool
	Solid       bool
	Level       int
	BlockSize   int // exponent 14–32, default 20
	Threads     int
	Multi       int
	Output      string

	Overwrite       bool
	RmOldVersion    bool
	RmSource        bool
	ParseCnmt       bool
	AlwaysParseCnmt bool
	Depth           int
	ExtractRegex    string

	Undupe            bool
	UndupeDryRun      bool
	UndupeRename      bool
	UndupeHardlink    bool
	UndupePriority    string
	UndupeWhitelist   string
	UndupeBlacklist   string
	UndupeOldVersions bool

	Create string

	MachineReadable bool
	Verbose         bool
	Quiet           bool
}

// DefaultOptions returns CLI defaults aligned with Python argparse.
func DefaultOptions() Options {
	return Options{
		Level:       18,
		BlockSize:   20,
		Threads:     -1,
		Multi:       4,
		Depth:       1,
		CompressXCI: false,
	}
}
