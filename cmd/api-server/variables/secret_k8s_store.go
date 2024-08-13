package variables

import (
	"encoding/hex"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretKey  = "SECRET_KEY"
	secretName = "secret-key"
)

type K8sSecretKeyStorage struct {
	namespace string
}

func NewK8sSecretKey(namespace string) SecretKeyStorage {
	return &K8sSecretKeyStorage{
		namespace: namespace,
	}
}

func (l *K8sSecretKeyStorage) Generate() error {
	key, err := GenerateAESKey(256)
	if err != nil {
		return err
	}

	// Save the key
	err = l.Save(key)
	if err != nil {
		return err
	}
	return nil
}

func (l *K8sSecretKeyStorage) Save(key []byte) error {
	// Save the key using the k8s secret as the storage
	encodedKey := hex.EncodeToString(key)
	data := map[string]string{
		secretKey: encodedKey,
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Immutable:  nil,
		StringData: data,
		Type:       "Opaque",
	}

	k8sClient, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}

	err = k8sUtils.CreateSecret(k8sClient, l.namespace, &secret)
	if err != nil {
		return err
	}
	return nil
}

func (l *K8sSecretKeyStorage) Load() ([]byte, error) {
	// Read the encoded key from the k8s secret
	k8sClient, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, err
	}

	k8sSecret, err := k8sUtils.GetSecret(k8sClient, l.namespace, secretName)
	if err != nil {
		return nil, err
	}

	encodedKey := k8sSecret.Data[secretKey]
	// Decode the key from hex string back to binary
	return hex.DecodeString(string(encodedKey))
}
