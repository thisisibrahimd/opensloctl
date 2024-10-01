package generator

type GeneratedFile struct {
	Name string
	Data []byte
}

type Generator interface {
	Generate(outputDirectory string) error
}
