package buildtool

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/storage"
)

// AzureBlobstoreConfig is an authentication and configuration struct containing
// the data needed by the Azure SDK to interact with a speicifc container in the
// blobstore.
type AzureBlobstoreConfig struct {
	Account   string // Account name to authorize API requests with
	Token     string // Access token for the above account
	Container string // Blob container to upload files into
}

// AzureBlobstoreUpload uploads a local file to the Azure Blob Storage. Note, this
// method assumes a max file size of 64MB (Azure limitation). Larger files will
// need a multi API call approach implemented.
//
// See: https://msdn.microsoft.com/en-us/library/azure/dd179451.aspx#Anchor_3
func AzureBlobstoreUpload(path string, name string, config AzureBlobstoreConfig) error {
	if *DryRunFlag {
		fmt.Printf("would upload %q to %s/%s/%s\n", path, config.Account, config.Container, name)
		return nil
	}

	// Create an authenticated client against the Azure cloud
	rawClient, err := storage.NewBasicClient(config.Account, config.Token)
	if err != nil {
		return err
	}
	client := rawClient.GetBlobService()

	// Stream the file to upload into the designated blobstore container
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	return client.CreateBlockBlobFromReader(config.Container, name, uint64(info.Size()), in, nil)
}
