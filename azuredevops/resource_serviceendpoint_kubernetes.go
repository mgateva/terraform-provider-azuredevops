package azuredevops

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/serviceendpoint"
	crud "github.com/microsoft/terraform-provider-azuredevops/azuredevops/crud/serviceendpoint"

	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/utils/converter"
)

func resourceServiceEndpointKubernetes() *schema.Resource {
	r := crud.GenBaseServiceEndpointResource(flattenServiceEndpointKubernetes, expandServiceEndpointKubernetes)
	r.Schema["apiserver_url"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "URL to Kubernete's API-Server",
	}
	r.Schema["authorization_type"] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		Description:  "Type of credentials to use",
		ValidateFunc: validation.StringInSlice([]string{"AzureSubscription"}, false),
	}
	r.Schema["azure_subscription"] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'AzureSubscription'-type of configuration",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"azure_environment": {
					Type:        schema.TypeString,
					Required:    false,
					Optional:    true,
					Default:     "AzureCloud",
					Description: "type of azure cloud: AzureCloud",
				},
				"cluster_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of aks-resource",
				},
				"subscription_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of azure subscription",
				},
				"subscription_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of azure subscription",
				},
				"tenant_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of aad-tenant",
				},
				"resourcegroup_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of resourcegroup",
				},
				"namespace": {
					Type:        schema.TypeString,
					Required:    true,
					Default:     "default",
					Description: "accessed namespace",
				},
			},
		},
	}
	return r
}

// Convert internal Terraform data structure to an AzDO data structure
func expandServiceEndpointKubernetes(d *schema.ResourceData) (*serviceendpoint.ServiceEndpoint, *string, error) {
	serviceEndpoint, projectID := crud.DoBaseExpansion(d)
	serviceEndpoint.Type = converter.String("kubernetes")
	serviceEndpoint.Url = converter.String(d.Get("apiserver_url").(string))

	switch d.Get("authorization_type").(string) {
	case "AzureSubscription":
		configurationRaw := d.Get("azure_subscription").(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"azureEnvironment": configuration["azure_environment"].(string),
				"azureTenantId":    configuration["tenant_id"].(string),
			},
			Scheme: converter.String("Kubernetes"),
		}

		clusterId := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ContainerService/managedClusters/%s", configuration["subscription_id"].(string), configuration["resourcegroup_id"].(string), configuration["cluster_name"].(string))
		serviceEndpoint.Data = &map[string]string{
			"authorizationType":     "AzureSubscription",
			"azureSubscriptionId":   configuration["subscription_id"].(string),
			"azureSubscriptionName": configuration["subscription_name"].(string),
			"clusterId":             clusterId,
			"namespace":             configuration["namespace"].(string),
		}
	}

	return serviceEndpoint, projectID, nil
}

// Convert AzDO data structure to internal Terraform data structure
func flattenServiceEndpointKubernetes(d *schema.ResourceData, serviceEndpoint *serviceendpoint.ServiceEndpoint, projectID *string) {
	crud.DoBaseFlattening(d, serviceEndpoint, projectID)
	d.Set("azure_environment", (*serviceEndpoint.Authorization.Parameters)["azureEnvironment"])
	d.Set("tenant_id", (*serviceEndpoint.Authorization.Parameters)["azureTenantId"])
	d.Set("authorization_type", (*serviceEndpoint.Data)["authorizationType"])
	d.Set("subscription_id", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionId"])
	d.Set("subscription_name", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionName"])
	d.Set("namespace", (*serviceEndpoint.Authorization.Parameters)["namespace"])
}
