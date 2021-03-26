// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"log"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	inmemory "github.com/patrickmn/go-cache"
)

type TokenCache struct {
	cache *inmemory.Cache
}

func (t *TokenCache) Replace(cache cache.Unmarshaler, key string) {
	data, found := t.cache.Get(key)
	if !found {
		//log.Println("Dint find item in cache")
	}
	buf, ok := data.([]byte)
	if !ok {
		//fmt.Println("byte conversion didnt work as expected")
	}

	err := cache.Unmarshal(buf)
	if err != nil {
		log.Println(err)
	}
}

func (t *TokenCache) Export(cache cache.Marshaler, key string) {
	data, err := cache.Marshal()
	if err != nil {
		log.Println(err)
	}
	t.cache.Set(key, data, -1)
}
