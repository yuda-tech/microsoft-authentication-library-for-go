// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/base"
	internalTime "github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/json/types/time"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/oauth"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/oauth/fake"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/oauth/ops/accesstokens"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/oauth/ops/authority"
	inmemory "github.com/patrickmn/go-cache"
)

var cacheAccessor = &TokenCache{cache: inmemory.New(5*time.Minute, 10*time.Minute)}

type testParameters struct {
	// the number of tenants to use
	TenantCount int

	// the number of tokens in the cache
	// must be divisible by Concurrency
	TokenCount int
}

func fakeClientwithTenantId(tenantID string) (base.Client, error) {
	// we use a base.Client so we can provide a fake OAuth client
	return base.New("fake_client_id", "https://fake_authority/"+tenantID, &oauth.Client{
		AccessTokens: &fake.AccessTokens{
			AccessToken: accesstokens.TokenResponse{
				AccessToken:   accessToken,
				ExpiresOn:     internalTime.DurationTime{T: time.Now().Add(1 * time.Hour)},
				GrantedScopes: accesstokens.Scopes{Slice: tokenScope},
			},
		},
		Authority: &fake.Authority{
			InstanceResp: authority.InstanceDiscoveryResponse{
				Metadata: []authority.InstanceDiscoveryMetadata{
					{
						PreferredNetwork: "fake_authority",
						Aliases:          []string{"fake_authority"},
					},
				},
			},
		},
		Resolver: &fake.ResolveEndpoints{
			Endpoints: authority.Endpoints{
				AuthorizationEndpoint: "auth_endpoint",
				TokenEndpoint:         "token_endpoint",
			},
		},
		WSTrust: &fake.WSTrust{},
	}, base.WithCacheAccessor(cacheAccessor))
}

type executionTime struct {
	start          time.Time
	end            time.Time
	durationValues []time.Duration
}

func populateTokenCachePerPartition(params testParameters, durationValuesPopulate []time.Duration) executionTime {
	fmt.Printf("Populating token cache with %d tokens...", params.TokenCount)
	start := time.Now()
	for i := 0; i < params.TokenCount; i++ {
		start1 := time.Now()
		client, err := fakeClientwithTenantId(strconv.FormatInt(int64(i%(params.TenantCount)), 10))
		if err != nil {
			panic(err)
		}
		authParams := client.AuthParams
		authParams.Scopes = tokenScope
		authParams.AuthorizationType = authority.ATClientCredentials
		// we use this to add a fake token to the cache.
		// each token has a different scope which is what makes them unique
		_, err = client.AuthResultFromToken(context.Background(), authParams, accesstokens.TokenResponse{
			AccessToken:   accessToken,
			ExpiresOn:     internalTime.DurationTime{T: time.Now().Add(1 * time.Hour)},
			GrantedScopes: accesstokens.Scopes{Slice: []string{strconv.FormatInt(int64(i), 10)}},
		}, true)
		if err != nil {
			panic(err)
		}
		end1 := time.Now()
		durationValuesPopulate[i] = end1.Sub(start1)
	}
	return executionTime{start: start, end: time.Now(), durationValues: durationValuesPopulate}
}

func executeTestWithPartitions(params testParameters, durationValues []time.Duration) executionTime {
	fmt.Printf("Begin token retrieval.....")
	start := time.Now()
	for i := 0; i < params.TokenCount; i++ {
		start1 := time.Now()
		client, err := fakeClientwithTenantId(strconv.FormatInt(int64(i%(params.TenantCount)), 10))
		if err != nil {
			fmt.Println("Failed while creating a client")
			panic(err)
		}
		_, err = client.AcquireTokenSilent(context.Background(), base.AcquireTokenSilentParameters{
			Scopes:      []string{strconv.FormatInt(int64(i), 10)},
			RequestType: accesstokens.ATConfidential,
			Credential: &accesstokens.Credential{
				Secret: "fake_secret",
			},
			IsAppCache: true,
		})
		if err != nil {
			fmt.Println(err)
		}
		end1 := time.Now()
		durationValues[i] = end1.Sub(start1)

	}
	return executionTime{start: start, end: time.Now(), durationValues: durationValues}
}

// PerfStats is used with statsTemplText for reporting purposes
type PerfStats struct {
	popExec executionTime
	retExec executionTime
	Tenants int
	Count   int64
}

// PopDur returns the total duration for populating the cache.
func (s *PerfStats) PopDur() time.Duration {
	return s.popExec.end.Sub(s.popExec.start)
}

// RetDur returns the total duration for retrieving tokens.
func (s *PerfStats) RetDur() time.Duration {
	return s.retExec.end.Sub(s.retExec.start)
}

// PopAvg returns the mean average of caching a token.
func (s *PerfStats) PopAvg() time.Duration {
	return s.PopDur() / time.Duration(s.Count)
}

// RetAvg returns the mean average of retrieving a token.
func (s *PerfStats) RetAvg() time.Duration {
	return s.RetDur() / time.Duration(s.Count)
}

var statsTemplTxt = `
Test Results:
[{{.Tenants}} tenants][{{.Count}} tokens] [population: total {{.PopDur}}, avg {{.PopAvg}}] [retrieval: total {{.RetDur}}, avg {{.RetAvg}}]
==========================================================================
`
var statsTemplate = template.Must(template.New("stats").Parse(statsTemplTxt))

func TestPerformance() {
	t := testParameters{
		TenantCount: 100,
		TokenCount:  400,
	}
	var durationValuesPopulate = make([]time.Duration, t.TokenCount)
	var durationValues = make([]time.Duration, t.TokenCount)

	fmt.Printf("Test Params: %#v\n", t)

	ptime := populateTokenCachePerPartition(t, durationValuesPopulate)
	ttime := executeTestWithPartitions(t, durationValues)
	if err := statsTemplate.Execute(os.Stdout, &PerfStats{
		popExec: ptime,
		retExec: ttime,
		Tenants: t.TenantCount,
		Count:   int64(t.TokenCount),
	}); err != nil {
		panic(err)
	}
	fmt.Println("Populate Statistic")

	sort.Slice(ptime.durationValues, func(i, j int) bool { return ptime.durationValues[i] < ptime.durationValues[j] })
	fmt.Println("P50", ptime.durationValues[int((0.5*(float64(t.TokenCount)))+0.5)])
	fmt.Println("P95", ptime.durationValues[int((0.95*(float64(t.TokenCount)))+0.5)])

	fmt.Println("Retreive Statistic")
	sort.Slice(ttime.durationValues, func(i, j int) bool { return ttime.durationValues[i] < ttime.durationValues[j] })
	fmt.Println("P50", ttime.durationValues[int((0.5*(float64(t.TokenCount)))+0.5)])
	fmt.Println("P95", ttime.durationValues[int((0.95*(float64(t.TokenCount)))+0.5)])
}
