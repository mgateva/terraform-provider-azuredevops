package azuredevops

import (
	"fmt"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/utils/tfhelper"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/utils/validate"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/serviceendpoint"
	crud "github.com/microsoft/terraform-provider-azuredevops/azuredevops/crud/serviceendpoint"

	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/utils/converter"
)

const (
	resourceAttrAuthType            = "authorization_type"
	resourceAttrAPIURL              = "apiserver_url"
	resourceBlockAzSubscription     = "azure_subscription"
	resourceBlockKubeconfig         = "kubeconfig"
	resourceBlockServiceAccount     = "service_account"
	serviceEndpointDataAttrAuthType = "authorizationType"
)

func makeSchemaAzureSubscription(r *schema.Resource) {
	r.Schema[resourceBlockAzSubscription] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'AzureSubscription'-type of configuration",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"azure_environment": {
					Type:         schema.TypeString,
					Optional:     true,
					Default:      "AzureCloud",
					Description:  "type of azure cloud: AzureCloud",
					ValidateFunc: validation.StringInSlice([]string{"AzureCloud"}, false),
				},
				"cluster_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of aks-resource",
				},
				"subscription_id": {
					Type:         schema.TypeString,
					Required:     true,
					Description:  "id of azure subscription",
					ValidateFunc: validation.IsUUID,
				},
				"subscription_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of azure subscription",
				},
				"tenant_id": {
					Type:         schema.TypeString,
					Required:     true,
					Description:  "id of aad-tenant",
					ValidateFunc: validation.IsUUID,
				},
				"resourcegroup_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of resourcegroup",
				},
				"namespace": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "default",
					Description: "accessed namespace",
				},
			},
		},
	}
}

func makeSchemaKubeconfig(r *schema.Resource) {
	resourceElemSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"cluster_context": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Context of your cluster",
			},
			"accept_untrusted_certs": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Enable this if your authentication uses untrusted certificates",
			},
		},
	}
	crud.MakeProtectedSchema(resourceElemSchema, "kube_config", "AZDO_KUBERNETES_SERVICE_CONNECTION_KUBECONFIG", "Content of the kubeconfig file. The configuration information in your kubeconfig file allows Kubernetes clients to talk to your Kubernetes API servers. This file is used by kubectl and all supported Kubernetes clients.")
	r.Schema[resourceBlockKubeconfig] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'Kubeconfig'-type of configuration",
		Elem:        resourceElemSchema,
	}
}

func makeSchemaServiceAccount(r *schema.Resource) {
	tokenHashKey, tokenHashSchema := tfhelper.GenerateSecreteMemoSchema("token")
	certHashKey, certHashSchema := tfhelper.GenerateSecreteMemoSchema("ca_cert")

	resourceElemSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"ca_cert": {
				Type:         schema.TypeString,
				Sensitive:    true,
				Optional:     true,
				Description:  "Service account certificate",
				ValidateFunc: validate.NoEmptyStrings,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return true
				},
			},
			"token": {
				Type:         schema.TypeString,
				Sensitive:    true,
				Optional:     true,
				Description:  "Token",
				ValidateFunc: validate.NoEmptyStrings,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return true
				},
			},
			tokenHashKey: tokenHashSchema,
			certHashKey:  certHashSchema,
		},
	}
	/*patHashKey, patHashSchema := tfhelper.GenerateSecreteMemoSchema("ca_cert")
	patHashKeyT, patHashSchemaT := tfhelper.GenerateSecreteMemoSchema("token")
	resourceElemSchema.Schema[patHashKey] = patHashSchema
	resourceElemSchema.Schema[patHashKeyT] = patHashSchemaT
	crud.MakeProtectedSchema(resourceElemSchema, "ca_cert", "AZDO_KUBERNETES_SERVICE_CONNECTION_SERVICE_ACCOUNT_CERT", "Secret cert")
	crud.MakeProtectedSchema(resourceElemSchema, "token", "AZDO_KUBERNETES_SERVICE_CONNECTION_SERVICE_ACCOUNT_TOKEN", "Secret token")*/
	r.Schema[resourceBlockServiceAccount] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'ServiceAccount'-type of configuration",
		Elem:        resourceElemSchema,
	}
}

func resourceServiceEndpointKubernetes() *schema.Resource {
	r := crud.GenBaseServiceEndpointResource(flattenServiceEndpointKubernetes, expandServiceEndpointKubernetes, parseImportedProjectIDAndServiceEndpointID)
	r.Schema[resourceAttrAPIURL] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		Description:  "URL to Kubernete's API-Server",
		ValidateFunc: validation.IsURLWithHTTPorHTTPS,
	}
	r.Schema[resourceAttrAuthType] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		Description:  "Type of credentials to use",
		ValidateFunc: validation.StringInSlice([]string{"AzureSubscription", "Kubeconfig", "ServiceAccount"}, false),
	}
	makeSchemaAzureSubscription(r)
	makeSchemaKubeconfig(r)
	makeSchemaServiceAccount(r)

	return r
}

// Convert internal Terraform data structure to an AzDO data structure
func expandServiceEndpointKubernetes(d *schema.ResourceData) (*serviceendpoint.ServiceEndpoint, *string, error) {
	serviceEndpoint, projectID := crud.DoBaseExpansion(d)
	serviceEndpoint.Type = converter.String("kubernetes")
	serviceEndpoint.Url = converter.String(d.Get(resourceAttrAPIURL).(string))

	switch d.Get(resourceAttrAuthType).(string) {
	case "AzureSubscription":
		configurationRaw := d.Get(resourceBlockAzSubscription).(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})
		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"azureEnvironment": configuration["azure_environment"].(string),
				"azureTenantId":    configuration["tenant_id"].(string),
			},
			Scheme: converter.String("Kubernetes"),
		}

		clusterID := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ContainerService/managedClusters/%s", configuration["subscription_id"].(string), configuration["resourcegroup_id"].(string), configuration["cluster_name"].(string))
		serviceEndpoint.Data = &map[string]string{
			"authorizationType":     "AzureSubscription",
			"azureSubscriptionId":   configuration["subscription_id"].(string),
			"azureSubscriptionName": configuration["subscription_name"].(string),
			"clusterId":             clusterID,
			"namespace":             configuration["namespace"].(string),
		}
	case "Kubeconfig":
		configurationRaw := d.Get(resourceBlockKubeconfig).(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})

		clusterContextInput := configuration["cluster_context"].(string)
		if clusterContextInput == "" {
			kubeConfigYAML := configuration["kube_config"].(string)
			var kubeConfigYAMLUnmarshalled map[string]interface{}
			err := yaml.Unmarshal([]byte(kubeConfigYAML), &kubeConfigYAMLUnmarshalled)
			if err != nil {
				errResult := fmt.Errorf("kube_config contains an invalid YAML: %s", err)
				return nil, nil, errResult
			}
			clusterContextInputList := kubeConfigYAMLUnmarshalled["contexts"].([]interface{})[0].(map[interface{}]interface{})
			clusterContextInput = clusterContextInputList["name"].(string)
		}

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"clusterContext": clusterContextInput,
				"kubeconfig":     configuration["kube_config"].(string),
			},
			Scheme: converter.String("Kubernetes"),
		}

		serviceEndpoint.Data = &map[string]string{
			"authorizationType":    "Kubeconfig",
			"acceptUntrustedCerts": fmt.Sprintf("%v", configuration["accept_untrusted_certs"].(bool)),
		}
	case "ServiceAccount":
		configurationRaw := d.Get(resourceBlockServiceAccount).(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})

		(*serviceEndpoint.Authorization.Parameters)["apiToken"] = expandSecret(configuration, "token")
		(*serviceEndpoint.Authorization.Parameters)["serviceAccountCertificate"] = expandSecret(configuration, "ca_cert")

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Scheme: converter.String("Token"),
		}

		serviceEndpoint.Data = &map[string]string{
			"authorizationType": "ServiceAccount",
		}
	}

	return serviceEndpoint, projectID, nil
}

func expandSecret(configuration map[string]interface{}, attr string) string {
	// Note: if this is an update for a field other than `serviceprincipalkey`, the `serviceprincipalkey` will be
	// set to `""`. Without catching this case and setting the value to `"null"`, the `serviceprincipalkey` will
	// actually be set to `""` by the Azure DevOps service.
	//
	// This step is critical in order to ensure that the service connection can update without loosing its password!
	//
	// This behavior is unfortunately not documented in the API documentation.
	secret, ok := configuration[attr]
	if !ok || secret.(string) == "" {
		return "null"
	}

	return secret.(string)
}

// Convert AzDO data structure to internal Terraform data structure
func flattenServiceEndpointKubernetes(d *schema.ResourceData, serviceEndpoint *serviceendpoint.ServiceEndpoint, projectID *string) {
	crud.DoBaseFlattening(d, serviceEndpoint, projectID)
	d.Set(resourceAttrAuthType, (*serviceEndpoint.Data)[serviceEndpointDataAttrAuthType])
	d.Set(resourceAttrAPIURL, (*serviceEndpoint.Url))

	switch (*serviceEndpoint.Data)[serviceEndpointDataAttrAuthType] {
	case "AzureSubscription":
		clusterIDSplit := strings.Split((*serviceEndpoint.Data)["clusterId"], "/")
		var clusterNameIndex int
		var resourceGroupIDIndex int
		for k, v := range clusterIDSplit {
			if v == "resourcegroups" {
				resourceGroupIDIndex = k + 1
			}
			if v == "managedClusters" {
				clusterNameIndex = k + 1
			}
		}
		configItems := map[string]interface{}{
			"azure_environment": (*serviceEndpoint.Authorization.Parameters)["azureEnvironment"],
			"tenant_id":         (*serviceEndpoint.Authorization.Parameters)["azureTenantId"],
			"subscription_id":   (*serviceEndpoint.Data)["azureSubscriptionId"],
			"subscription_name": (*serviceEndpoint.Data)["azureSubscriptionName"],
			"cluster_name":      clusterIDSplit[clusterNameIndex],
			"resourcegroup_id":  clusterIDSplit[resourceGroupIDIndex],
			"namespace":         (*serviceEndpoint.Data)["namespace"],
		}
		configItemList := make([]map[string]interface{}, 1)
		configItemList[0] = configItems

		d.Set(resourceBlockAzSubscription, configItemList)
	case "Kubeconfig":
		kubeconfigResource := &schema.Resource{
			Schema: map[string]*schema.Schema{},
		}
		makeSchemaKubeconfig(kubeconfigResource)

		tfhelper.HelpFlattenSecret(d, "kube_config")
		acceptUntrustedCerts, _ := strconv.ParseBool((*serviceEndpoint.Data)["acceptUntrustedCerts"])
		configItems := []interface{}{
			map[string]interface{}{
				"kube_config":            (*serviceEndpoint.Authorization.Parameters)["kubeconfig"],
				"cluster_context":        (*serviceEndpoint.Authorization.Parameters)["clusterContext"],
				"accept_untrusted_certs": acceptUntrustedCerts,
			},
		}

		kubeConfigSchemaSet := schema.NewSet(schema.HashResource(kubeconfigResource), configItems)
		d.Set(resourceBlockKubeconfig, kubeConfigSchemaSet)
	case "ServiceAccount":
		var serviceAccount map[string]interface{}
		serviceAccountSet := d.Get("service_account").(*schema.Set).List()
		if len(serviceAccountSet) > 0 {
			configuration := serviceAccountSet[0].(map[string]interface{})
			newHashToken, hashKeyToken := tfhelper.HelpFlattenSecretNested(d, resourceBlockServiceAccount, configuration, "token")
			newHashCert, hashKeyCert := tfhelper.HelpFlattenSecretNested(d, resourceBlockServiceAccount, configuration, "ca_cert")
			serviceAccount = map[string]interface{}{
				"token":      configuration["token"].(string),
				"ca_cert":    configuration["ca_cert"].(string),
				hashKeyToken: newHashToken,
				hashKeyCert:  newHashCert,
			}
		} else {
			serviceAccount = map[string]interface{}{
				"token":   (*serviceEndpoint.Authorization.Parameters)["apiToken"],
				"ca_cert": (*serviceEndpoint.Authorization.Parameters)["serviceAccountCertificate"],
			}
		}
		serviceAccountList := make([]map[string]interface{}, 1)
		serviceAccountList[0] = serviceAccount
		d.Set(resourceBlockServiceAccount, serviceAccountList)

		/*serviceAccountSet := d.Get(resourceBlockServiceAccount).(*schema.Set).List()
		if len(serviceAccountSet) == 1 {
			if serviceAccount,ok := serviceAccountSet[0].(map[string]interface{});ok {
				newHashToken, hashKeyToken := tfhelper.HelpFlattenSecretNested(d, resourceBlockServiceAccount, serviceAccount, "token")
				newHashCert, hashKeyCert := tfhelper.HelpFlattenSecretNested(d, resourceBlockServiceAccount, serviceAccount, "ca_cert")
				serviceAccount[hashKeyToken] = newHashToken
				serviceAccount[hashKeyCert] = newHashCert
				/*serviceAccountList := make([]map[string]interface{}, 1)
				serviceAccountList[0] = serviceAccount
				d.Set(resourceBlockServiceAccount, serviceAccountList)
				d.Set(resourceBlockServiceAccount, []interface{}{serviceAccount})
			}
		}*/
	}
}
