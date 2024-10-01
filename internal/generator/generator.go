package generator

type GeneratedFile struct {
	Path string
	Data string
}

func (f *GeneratedFile) Bytes() []byte {
	return []byte(f.Data)

}

type Generator interface {
	Generate(outputDirectory string) error
}
