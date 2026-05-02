package collector

// Collector interface and implementations for gathering investigation data

type Collector interface {
	Collect(outputDir string) error
}
