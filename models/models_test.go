package models

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/nyaruka/rp-clover/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func setUp(t *testing.T) *sqlx.DB {
	db, err := sqlx.Open("postgres", "postgres://localhost/clover_test?sslmode=disable")
	if err != nil {
		t.Fatalf("error connecting to db: %s", err)
	}

	db.Exec("drop table urn_mappings cascade;")
	db.Exec("drop table interchanges cascade;")
	db.Exec("drop table channels cascade;")
	db.Exec("drop table migrations;")
	err = migrations.Migrate(context.Background(), db)
	if err != nil {
		t.Fatalf("error migrating db: %s", err)
	}

	return db
}

func TestConfigs(t *testing.T) {
	db := setUp(t)

	tcs := []struct {
		input  string
		output string
		hasErr bool
	}{
		{`[]`, `[]`, false},
		{`[{}]`, `[]`, true},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			}
		]`,
			`[
		    {
		        "uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
		        "name": "Nigeria",
		        "country": "NE",
		        "scheme": "twitter",
		        "channels": [
		            {
		                "uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
		                "name": "U-Report Nigeria",
		                "url": "https://foobar",
		                "keywords": [
		                    "one"
		                ]
		            }
		        ]
		    }
		]`, false},
		{`[]`, `[]`, false},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": []
			}
		]`, `[]`, true},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": []
			}
		]`, `[]`, true},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					},
					{
						"uuid": "09057743-f615-4b5c-bd58-e87074f38aaa",
						"name": "U-Report Nigeria NE",
						"url": "https://foobar",
						"keywords": [
							"two", "three"
						]
					}
				]
			}
		]`,
			`[
		    {
		        "uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
		        "name": "Nigeria",
		        "country": "NE",
		        "scheme": "twitter",
		        "channels": [
		            {
		                "uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
		                "name": "U-Report Nigeria",
		                "url": "https://foobar",
		                "keywords": [
		                    "one"
		                ]
		            },
					{
						"uuid": "09057743-f615-4b5c-bd58-e87074f38aaa",
						"name": "U-Report Nigeria NE",
						"url": "https://foobar",
						"keywords": [
							"two", "three"
						]
					}
		        ]
		    }
		]`, false},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "09057743-f615-4b5c-bd58-e87074f38aaa",
						"name": "U-Report Nigeria NE",
						"url": "https://foobar",
						"keywords": [
							"two", "three"
						]
					},
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			}
		]`,
			`[
				{
					"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
					"name": "Nigeria",
					"country": "NE",
					"scheme": "twitter",
					"channels": [
						{
							"uuid": "09057743-f615-4b5c-bd58-e87074f38aaa",
							"name": "U-Report Nigeria NE",
							"url": "https://foobar",
							"keywords": [
								"two", "three"
							]
						},
						{
							"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
							"name": "U-Report Nigeria",
							"url": "https://foobar",
							"keywords": [
								"one"
							]
						}
					]
				}
			]`, false},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			}
		]`,
			`[
				{
					"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
					"name": "Nigeria",
					"country": "NE",
					"scheme": "twitter",
					"channels": [
						{
							"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
							"name": "U-Report Nigeria",
							"url": "https://foobar",
							"keywords": [
								"one"
							]
						}
					]
				}
			]`, false},
		{`[]`, `[]`, false},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			},
			{
				"uuid": "afc2532c-1565-4016-a83e-fc6bc1ac3550",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			}
		]`, `[]`, true},
		{`[
			{
				"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
				"name": "Nigeria",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			},
			{
				"uuid": "afc2532c-1565-4016-a83e-fc6bc1ac3550",
				"name": "Nigeria NE",
				"country": "NE",
				"scheme": "twitter",
				"channels": [
					{
						"uuid": "7331140b-2be0-4855-92e1-fd06ca456364",
						"name": "U-Report Nigeria",
						"url": "https://foobar",
						"keywords": [
							"one"
						]
					}
				]
			}
		]`, `[
		    {
		        "uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
		        "name": "Nigeria",
		        "country": "NE",
		        "scheme": "twitter",
		        "channels": [
		            {
		                "uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
		                "name": "U-Report Nigeria",
		                "url": "https://foobar",
		                "keywords": [
		                    "one"
		                ]
		            }
		        ]
		    },
		    {
		        "uuid": "afc2532c-1565-4016-a83e-fc6bc1ac3550",
		        "name": "Nigeria NE",
		        "country": "NE",
		        "scheme": "twitter",
		        "channels": [
		            {
		                "uuid": "7331140b-2be0-4855-92e1-fd06ca456364",
		                "name": "U-Report Nigeria",
		                "url": "https://foobar",
		                "keywords": [
		                    "one"
		                ]
		            }
		        ]
		    }
		]`, false},
	}

	ctx := context.Background()
	for i, tc := range tcs {
		interchanges := make([]*Interchange, 0)
		err := json.Unmarshal([]byte(tc.input), &interchanges)
		if err != nil {
			t.Errorf("test %d, received error unmarshalling input: %s", i, tc.input)
			continue
		}

		err = UpdateInterchangeConfig(ctx, db, interchanges)
		if err == nil && tc.hasErr {
			t.Errorf("test %d, expected error got none", i)
		} else if err != nil && !tc.hasErr {
			t.Errorf("test %d, expected no error error, got '%s'", i, err)
		}

		// get our current config
		current, err := GetInterchangeConfig(ctx, db)
		assert.NoError(t, err)

		output, err := json.MarshalIndent(current, "", "    ")
		assert.NoError(t, err)

		cOutput := new(bytes.Buffer)
		json.Compact(cOutput, output)

		tcOutput := new(bytes.Buffer)
		json.Compact(tcOutput, []byte(tc.output))

		if cOutput.String() != tcOutput.String() {
			t.Errorf("test %d, expected output of :'%s' got: '%s'", i, tc.output, output)
		}
	}

	// one off loading of interchange
	interchange, err := GetInterchange(ctx, db, "afc2532c-1565-4016-a83e-fc6bc1ac3550")
	assert.NoErrorf(t, err, "error loading interchange")
	assert.NotNil(t, interchange, "got nil exchange back")
	assert.Equal(t, "Nigeria NE", interchange.Name)
	assert.Equal(t, 1, len(interchange.Channels))
}

func TestMappings(t *testing.T) {
	db := setUp(t)
	ctx := context.Background()

	config := `
	[
		{
			"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
			"name": "Nigeria",
			"country": "NE",
			"scheme": "tel",
			"channels": [
				{
					"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
					"name": "U-Report Nigeria",
					"url": "https://foobar",
					"keywords": [
						"one"
					]
				},
				{
					"uuid": "557d3353-6b89-441a-aee5-8c398fd7a62f",
					"name": "U-Report Nigeria NE",
					"url": "https://foobar",
					"keywords": [
						"two"
					]
				}
			]
		},
		{
			"uuid": "afc2532c-1565-4016-a83e-fc6bc1ac3550",
			"name": "Nigeria NE",
			"country": "NE",
			"scheme": "tel",
			"channels": [
				{
					"uuid": "7331140b-2be0-4855-92e1-fd06ca456364",
					"name": "U-Report Nigeria",
					"url": "https://foobar",
					"keywords": [
						"one"
					]
				}
			]
		}
	]`

	interchanges := make([]*Interchange, 0)
	err := json.Unmarshal([]byte(config), &interchanges)
	assert.NoErrorf(t, err, "received error unmarshalling config")

	err = UpdateInterchangeConfig(ctx, db, interchanges)
	assert.NoErrorf(t, err, "received error writing config")

	interchanges, err = GetInterchangeConfig(ctx, db)
	assert.NoError(t, err)

	i1 := interchanges[0]
	i1c1 := &interchanges[0].Channels[0]
	i1c2 := &interchanges[0].Channels[1]
	i2 := interchanges[1]
	i2c1 := &interchanges[1].Channels[0]

	tcs := []struct {
		interchange     *Interchange
		channel         *Channel
		urn             string
		testInterchange *Interchange
		testURN         string
		testChannel     *Channel
	}{
		{i1, i1c1, "tel:2065551212", i1, "tel:2065551212", i1c1},
		{i1, i1c1, "tel:2065551212", i1, "tel:2065551213", nil},
		{i1, i1c2, "tel:2065551212", i1, "tel:2065551212", i1c2},
		{i1, nil, "tel:2065551212", i1, "tel:2065551212", nil},
		{i2, i2c1, "tel:2065551213", i2, "tel:2065551213", i2c1},
		{i2, i2c1, "tel:2065551213", i1, "tel:2065551213", nil},
	}

	for i, tc := range tcs {
		if tc.channel != nil {
			err := SetChannelForURN(ctx, db, tc.interchange, tc.channel, tc.urn)
			assert.NoErrorf(t, err, "test %d: error setting channel", i)
		} else {
			err := ClearChannelForURN(ctx, db, tc.interchange, tc.urn)
			assert.NoErrorf(t, err, "test %d: error clearing channel", i)
		}

		channel, err := GetChannelForURN(ctx, db, tc.testInterchange, tc.testURN)
		assert.NoErrorf(t, err, "test %d: error getting channel", i)

		if tc.testChannel != nil && channel != nil {
			assert.Equalf(t, tc.testChannel.UUID, channel.UUID, "test %d: channel is not as expected", i)
		} else {
			assert.Equalf(t, tc.testChannel, channel, "test %d: channel is not as expected", i)
		}
	}
}
