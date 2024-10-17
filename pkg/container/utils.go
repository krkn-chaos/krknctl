package container

import (
	"encoding/base64"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
)

func encodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func FlattenKubeconfig() {
	// Carica il file kubeconfig
	configPath := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err := clientcmd.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Errore caricando il kubeconfig: %v", err)
	}

	// Itera sui contesti e appiattisce le configurazioni
	for clusterName, cluster := range config.Clusters {
		if cluster.CertificateAuthorityData == nil && cluster.CertificateAuthority != "" {
			certData, err := os.ReadFile(cluster.CertificateAuthority)
			if err != nil {
				log.Fatalf("Errore leggendo il certificato del cluster '%s': %v", clusterName, err)
			}
			cluster.CertificateAuthorityData = certData
			cluster.CertificateAuthority = ""
		}
	}

	for authName, auth := range config.AuthInfos {
		if auth.ClientCertificateData == nil && auth.ClientCertificate != "" {
			certData, err := os.ReadFile(auth.ClientCertificate)
			if err != nil {
				log.Fatalf("Errore leggendo il certificato dell'utente '%s': %v", authName, err)
			}
			auth.ClientCertificateData = certData
			auth.ClientCertificate = ""
		}

		if auth.ClientKeyData == nil && auth.ClientKey != "" {
			keyData, err := os.ReadFile(auth.ClientKey)
			if err != nil {
				log.Fatalf("Errore leggendo la chiave dell'utente '%s': %v", authName, err)
			}
			auth.ClientKeyData = keyData
			auth.ClientKey = ""
		}
	}

	// Crea una configurazione piatta (flattend)
	flattenedConfig, err := clientcmd.Write(*config)
	if err != nil {
		log.Fatalf("Errore scrivendo il kubeconfig flattened: %v", err)
	}

	// Stampa la configurazione
	fmt.Println(string(flattenedConfig))
}
