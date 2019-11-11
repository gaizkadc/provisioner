/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package azure

import (
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/nalej/derrors"
	"github.com/rs/zerolog/log"
)

//const AzureBaseURI = "https://management.azure.com"
const AzureBaseURI = "https://graph.windows.net"
const GraphBaseURI = "https://graph.windows.net"
const ManagementBaseURI = "https://management.azure.com"

func GetGraphAuthorizer(credentials *AzureCredentials) (autorest.Authorizer, derrors.Error) {
	return GetAuthorizer(credentials, GraphBaseURI)
}

func GetManagementAuthorizer(credentials *AzureCredentials) (autorest.Authorizer, derrors.Error) {
	return GetAuthorizer(credentials, ManagementBaseURI)
}

func GetAuthorizer(credentials *AzureCredentials, targetURI string) (autorest.Authorizer, derrors.Error) {
	// This code is similar to the NewAuthorizerFromFile method, but we take the values from our structure.
	settings := auth.FileSettings{
		Values: make(map[string]string, 0),
	}

	settings.Values[auth.ClientID] = credentials.ClientId
	settings.Values[auth.ClientSecret] = credentials.ClientSecret
	// Note that in the API, the values of CertificatePath and CertificatePassword are set, but the file
	// resulting from the az command did not produced any of those values.
	settings.Values[auth.SubscriptionID] = credentials.SubscriptionId
	settings.Values[auth.TenantID] = credentials.TenantId
	settings.Values[auth.ActiveDirectoryEndpoint] = credentials.ActiveDirectoryEndpointUrl
	settings.Values[auth.ResourceManagerEndpoint] = credentials.ResourceManagerEndpointUrl
	settings.Values[auth.ActiveDirectoryEndpoint] = credentials.ActiveDirectoryEndpointUrl
	settings.Values[auth.SQLManagementEndpoint] = credentials.SqlManagementEndpointUrl
	settings.Values[auth.GalleryEndpoint] = credentials.GalleryEndpointUrl
	settings.Values[auth.ManagementEndpoint] = credentials.ManagementEndpointUrl
	settings.Values[auth.GraphResourceID] = GraphBaseURI

	auth, err := settings.ClientCredentialsAuthorizer(targetURI)
	if err == nil {
		return auth, nil
	}
	log.Error().Str("err", err.Error()).Msg("cannot create client with credentials")
	return nil, derrors.NewInternalError("auth file missing client and certificate credentials", err)
}

func GetBearerAuthorizer(credentials *AzureCredentials) (autorest.Authorizer, derrors.Error) {
	oauthConfig, err := adal.NewOAuthConfig(
		credentials.ActiveDirectoryEndpointUrl, credentials.TenantId)
	if err != nil {
		return nil, derrors.NewInternalError("cannot create OAuthConfig ", err)
	}

	token, err := adal.NewServicePrincipalToken(
		*oauthConfig, credentials.ClientId, credentials.ClientSecret, GraphBaseURI)
	log.Debug().Interface("token", token).Msg("Oauth token")
	if err != nil {
		return nil, derrors.NewInternalError("cannot create service principal token", err)
	}
	a := autorest.NewBearerAuthorizer(token)
	return a, nil
}
