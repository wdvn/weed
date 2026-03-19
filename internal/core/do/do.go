package do

type DataObject interface {
	Marshal() ([]byte, error)
	Unmarshal(data []byte, target any) error
}

type JsonInput struct{}
type FormInput struct {
}
type QueryInput struct {
}
