// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package msalbase

//StorageTokenResponse mimics a token response that was pulled from the cache
type StorageTokenResponse struct {
	accessToken  accessTokenInterfacer
	RefreshToken Credential
	idToken      Credential
	account      *Account
}

//CreateStorageTokenResponse creates a token response from cache
func CreateStorageTokenResponse(accessToken accessTokenInterfacer, refreshToken Credential, idToken Credential, account *Account) *StorageTokenResponse {
	tr := &StorageTokenResponse{accessToken, refreshToken, idToken, account}
	return tr
}
