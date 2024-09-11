package timoni

type Image struct {
	OCIRepository string `json:"ociRegistry"`
	Tag           string `json:"tag"`
	Credentials   string `json:"credentials"`
}

func NewImage(ociRegistry string, tag string, credentials string) *Image {
	return &Image{
		OCIRepository: ociRegistry,
		Tag:           tag,
		Credentials:   credentials,
	}
}
