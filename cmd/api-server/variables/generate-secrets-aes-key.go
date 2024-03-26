package variables

func CreateLocalSecretKey() {
	v := NewLocalSecretKey("secret.key")
	_ = v.Generate()
}
