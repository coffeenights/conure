package applications

//
//import (
//	"context"
//	"encoding/json"
//	"fmt"
//	k8sUtils "github.com/coffeenights/conure/internal/k8s"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/runtime/schema"
//	"k8s.io/apimachinery/pkg/watch"
//	"k8s.io/client-go/dynamic"
//	"log"
//	"testing"
//)
//
//func TestServiceComponentStatus(t *testing.T) {
//	clientset, err := k8sUtils.GetClientset()
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	cd, err := clientset.Vela.CoreV1beta1().ComponentDefinitions("vela-system").Get(context.TODO(), "webservice", metav1.GetOptions{})
//	if err != nil {
//		t.Fatal(err)
//	}
//	_ = cd
//	configmap, err := clientset.K8s.CoreV1().ConfigMaps("vela-system").Get(context.TODO(), "component-schema-webservice", metav1.GetOptions{})
//	if err != nil {
//		t.Fatal(err)
//	}
//	_ = configmap
//	var result map[string]interface{}
//	dataJSON := configmap.Data["openapi-v3-json-schema"]
//	data := json.Unmarshal([]byte(dataJSON), &result)
//	_ = data
//	gv := schema.GroupVersion{}
//	config := clientset.Config
//	config.GroupVersion = &gv
//
//	//scheme := runtime.NewScheme()
//	//codecFactory := serializer.NewCodecFactory(scheme)
//	//// Get a codec that performs conversion
//	//codec := codecFactory.LegacyCodec(gv)
//	//// Now you can use the codec for encoding and decoding
//	//config.NegotiatedSerializer = codec
//	//config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
//	//restClient, err := rest.RESTClientFor(clientset.Config)
//	//if err != nil {
//	//	t.Fatal(err)
//	//}
//	//r := restClient.Get()
//	//// r = r.AbsPath("api", "v1", "namespaces", "default", "pods")
//	//r = r.AbsPath("apis", "s3.aws.upbound.io", "v1beta1", "buckets")
//	//res := r.Do(context.TODO())
//	//
//	//if res.Error() != nil {
//	//	t.Fatal(res.Error())
//	//}
//	//decodedRes, err := res.Get()
//	//t.Log(decodedRes)
//
//	dynamicClient, err := dynamic.NewForConfig(config)
//	if err != nil {
//		t.Fatal(err)
//	}
//	gvr := schema.GroupVersionResource{
//		Group:    "s3.aws.upbound.io",
//		Version:  "v1beta1",
//		Resource: "buckets",
//	}
//	r1, err := dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
//	if err != nil {
//		t.Fatal(err)
//	}
//	t.Log(r1)
//
//}
//
//func TestChannel(_ *testing.T) {
//	watchlist := metav1.ListOptions{}
//	clientset, err := k8sUtils.GetClientset()
//	if err != nil {
//		log.Fatal(err)
//	}
//	pods, err := clientset.K8s.CoreV1().Pods("default").Watch(context.TODO(), watchlist)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	ch := pods.ResultChan()
//
//	for event := range ch {
//		pod, ok := event.Object.(*corev1.Pod)
//		if !ok {
//			log.Fatal("unexpected type")
//		}
//
//		switch event.Type {
//		case watch.Added:
//			fmt.Printf("Pod %s was added\n", pod.Name)
//		case watch.Modified:
//			fmt.Printf("Pod %s was modified\n", pod.Name)
//		case watch.Deleted:
//			fmt.Printf("Pod %s was deleted\n", pod.Name)
//		}
//	}
//}
