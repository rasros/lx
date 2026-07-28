module github.com/rasros/lx

go 1.25.7

require gopkg.in/yaml.v3 v3.0.1

require (
	github.com/atotto/clipboard v0.1.4
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728
	github.com/mholt/archives v0.1.5
	github.com/nguyenthenguyen/docx v0.0.0-20230621112118-9c8e795a11db
	// Held at v0.20.9, the last release that parses Go at a sane cost.
	// normalizeGoNewMakeTypeArgument walks a result tree that revisits its own
	// nodes, so the walk runs away on ordinary Go source: v0.21.0 and v0.24.0
	// through v0.47.0 overflow the stack outright, and v0.23.x survives only by
	// being some twenty times slower. A stack overflow is a fatal runtime error
	// rather than a panic, so it cannot be recovered from and takes the whole
	// process with it. Upgrading needs the walk fixed upstream first.
	github.com/odvcencio/gotreesitter v0.20.9
	github.com/xuri/excelize/v2 v2.11.0
	golang.org/x/net v0.57.0
)

require (
	github.com/STARRY-S/zip v0.2.3 // indirect
	github.com/andybalholm/brotli v1.2.2 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.5 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mikelolasagasti/xz v1.0.1 // indirect
	github.com/minio/minlz v1.2.0 // indirect
	github.com/nwaples/rardecode/v2 v2.3.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/richardlehane/mscfb v1.0.7 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/sorairolake/lzip-go v0.3.8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/stangelandcl/ppmd v0.1.1 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/ulikunitz/xz v0.5.16 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)
