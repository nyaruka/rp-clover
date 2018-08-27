package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	validator "gopkg.in/go-playground/validator.v9"
)

// Channel represents our channels
type Channel struct {
	UUID            string         `db:"uuid"              json:"uuid"      validate:"required,uuid4"`
	Name            string         `db:"name"              json:"name"      validate:"required"`
	InterchangeUUID string         `db:"interchange_uuid"  json:"-"`
	URL             string         `db:"url"               json:"url"       validate:"required,url"`
	Keywords        pq.StringArray `db:"keywords"          json:"keywords"`
}

// Interchange represents our interchanges
type Interchange struct {
	UUID               string    `db:"uuid"                  json:"uuid"     validate:"required,uuid4"`
	Name               string    `db:"name"                  json:"name"     validate:"required"`
	Country            string    `db:"country"               json:"country"  validate:"required"`
	Scheme             string    `db:"scheme"                json:"scheme"   validate:"required"`
	DefaultChannelUUID string    `db:"default_channel_uuid"  json:"-"`
	Channels           []Channel `                           json:"channels" validate:"required,dive"`

	// when we were loaded, for cache invalidation
	loadedOn time.Time
}

// URNMapping represents the mapping for a URN
type URNMapping struct {
	URN             string `db:"urn"              json:"urn"`
	InterchangeUUID string `db:"interchange_uuid" json:"interchange_uuid"`
	ChannelUUID     string `db:"channel_uuid"     json:"channel_uuid"`
}

const upsertInterchangeSQL = `
INSERT INTO interchanges (uuid, name, country, scheme, default_channel_uuid)
VALUES (:uuid, :name, :country, :scheme, :default_channel_uuid) 
ON CONFLICT (uuid) 
DO
 UPDATE
   SET name = :name, country = :country, scheme = :scheme, default_channel_uuid = :default_channel_uuid;
`

const upsertChannelSQL = `
INSERT INTO channels (uuid, name, interchange_uuid, url, keywords)
VALUES (:uuid, :name, :interchange_uuid, :url, :keywords) 
ON CONFLICT (uuid) 
DO
 UPDATE
   SET name = :name, interchange_uuid = :interchange_uuid, url = :url, keywords = :keywords;
`

// UpdateInterchangeConfig updates our interchange configs according to the passed in interchanges. Returns
// any errors encountered during validation or writing to the db.
func UpdateInterchangeConfig(ctx context.Context, db *sqlx.DB, interchanges []*Interchange) (err error) {
	err = validateInterchangeConfig(interchanges)
	if err != nil {
		return err
	}

	for _, interchange := range interchanges {
		// set our default channel UUID to the first channel
		interchange.DefaultChannelUUID = interchange.Channels[0].UUID
	}

	// ok this looks like it should work, do our updates in a single transaction
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// this will either rollback or commit based on our error state
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// for each interchange
	seenInterchanges := make(map[string]bool)
	seenChannels := make(map[string]bool)
	for _, interchange := range interchanges {
		// assign our default to our first channel
		interchange.DefaultChannelUUID = interchange.Channels[0].UUID
		seenInterchanges[interchange.UUID] = true

		// write this interchange
		_, err := tx.NamedExecContext(ctx, upsertInterchangeSQL, interchange)
		if err != nil {
			logrus.WithError(err).Error("error upserting interchange")
			return err
		}

		// write each channel
		for _, channel := range interchange.Channels {
			channel.InterchangeUUID = interchange.UUID
			seenChannels[channel.UUID] = true
			_, err = tx.NamedExecContext(ctx, upsertChannelSQL, channel)
			if err != nil {
				logrus.WithError(err).Error("error upserting interchange")
				return err
			}
		}
	}

	// if there are no interchanges, delete everything
	if len(interchanges) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM interchanges;`)
		if err != nil {
			return err
		}
	} else {
		// otherwise, remove all that we haven't seen
		interchangeUUIDs := mapKeys(seenInterchanges)
		_, err = tx.ExecContext(ctx, `DELETE FROM interchanges WHERE NOT ARRAY[uuid] <@ $1`, pq.Array(interchangeUUIDs))
		if err != nil {
			return err
		}

		// remove all channels we didn't see
		channelUUIDs := mapKeys(seenChannels)
		_, err = tx.ExecContext(ctx, `DELETE FROM channels WHERE NOT ARRAY[uuid] <@ $1`, pq.Array(channelUUIDs))
		if err != nil {
			return err
		}
	}

	// if we saved correctly clear out our cache
	if err == nil {
		cacheLock.Lock()
		interchangeCache = make(map[string]*Interchange)
		cacheLock.Unlock()
	}

	return err
}

// GetInterchangeConfig returns our complete interchange configuration
func GetInterchangeConfig(ctx context.Context, db *sqlx.DB) ([]*Interchange, error) {
	interchanges := make([]*Interchange, 0, 5)
	err := db.SelectContext(ctx, &interchanges, `SELECT * FROM interchanges ORDER BY uuid`)
	if err != nil {
		return nil, err
	}

	// load the channels for each interchange
	for i := range interchanges {
		channels, err := getChannelsForInterchange(ctx, db, interchanges[i])
		if err != nil {
			return nil, err
		}
		interchanges[i].Channels = channels
	}

	return interchanges, nil
}

// GetInterchange returns the interchange configuration for the passed in UUID. This will include the
// channels for the interchange with the default channel being the first channel in the slice.
func GetInterchange(ctx context.Context, db *sqlx.DB, uuid string) (*Interchange, error) {
	cacheLock.RLock()
	interchange, found := interchangeCache[uuid]
	cacheLock.RUnlock()

	// found it and loaded less than a minute ago? return it straight away
	if found && time.Now().Sub(interchange.loadedOn) < time.Minute {
		return interchange, nil
	}

	// allocate an interchange to load into
	interchange = &Interchange{}
	err := db.GetContext(ctx, interchange, `SELECT * FROM interchanges WHERE uuid = $1`, uuid)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		logrus.WithError(err).Error("error looking up interchange")
		return nil, err
	}

	channels, err := getChannelsForInterchange(ctx, db, interchange)
	if err != nil {
		logrus.WithError(err).Error("error looking up channels for interchange")
		return nil, err
	}

	interchange.Channels = channels
	interchange.loadedOn = time.Now()

	cacheLock.Lock()
	interchangeCache[uuid] = interchange
	cacheLock.Unlock()

	return interchange, nil
}

func getChannelsForInterchange(ctx context.Context, db *sqlx.DB, interchange *Interchange) ([]Channel, error) {
	channels := []Channel{}
	err := db.SelectContext(ctx, &channels, `SELECT * FROM channels WHERE interchange_uuid = $1 ORDER BY uuid`, interchange.UUID)

	// find the default channel and make it the first in the list
	found := false
	for i, channel := range channels {
		if channel.UUID == interchange.DefaultChannelUUID {
			tmp := channels[0]
			channels[0] = channel
			channels[i] = tmp
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("unable to find default channel: %s for interchange: %s", interchange.DefaultChannelUUID, interchange.UUID)
	}

	return channels, err
}

const upsertURNMappingSQL = `
INSERT INTO urn_mappings (interchange_uuid, channel_uuid, urn)
VALUES ($1, $2, $3) 
ON CONFLICT (interchange_uuid, urn) 
DO
 UPDATE
   SET channel_uuid = $2
`

// SetChannelForURN associates the passed in URN with the passed in Channel
func SetChannelForURN(ctx context.Context, db *sqlx.DB, interchange *Interchange, channel *Channel, urn string) error {
	// double check our channel membership
	if channel.InterchangeUUID != interchange.UUID {
		return fmt.Errorf("channel does not belong to interchange %s != %s", channel.InterchangeUUID, interchange.UUID)
	}

	_, err := db.ExecContext(ctx, upsertURNMappingSQL, interchange.UUID, channel.UUID, urn)
	if err != nil {
		logrus.WithError(err).Error("error upserting urn mapping")
	}

	return err
}

const getURNMappingSQL = `
SELECT c.uuid as uuid, c.name as name, c.interchange_uuid as interchange_uuid, c.url as url, c.keywords as keywords
FROM urn_mappings u, channels c
WHERE u.interchange_uuid = $1 AND u.urn = $2 AND u.channel_uuid = c.uuid
`

// GetChannelForURN returns the channel that is associated with the passed in URN, if any
func GetChannelForURN(ctx context.Context, db *sqlx.DB, interchange *Interchange, urn string) (*Channel, error) {
	channel := Channel{}
	err := db.GetContext(ctx, &channel, getURNMappingSQL, interchange.UUID, urn)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &channel, err
}

var (
	validate         = validator.New()
	interchangeCache = map[string]*Interchange{}
	cacheLock        = sync.RWMutex{}
)

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func validateInterchangeConfig(interchanges []*Interchange) error {
	// validate our interchanges as a whole
	seenInterchanges := make(map[string]bool)
	seenChannels := make(map[string]bool)

	for _, interchange := range interchanges {
		seenKeywords := make(map[string]bool)

		err := validateObject(interchange)
		if err != nil {
			return err
		}

		if seenInterchanges[interchange.UUID] {
			return fmt.Errorf("duplicate interchange UUID: %s", interchange.UUID)
		}
		seenInterchanges[interchange.UUID] = true

		for _, channel := range interchange.Channels {
			err = validateObject(channel)
			if err != nil {
				return err
			}

			if seenChannels[channel.UUID] {
				return fmt.Errorf("duplicate channel UUID: %s", channel.UUID)
			}
			seenChannels[channel.UUID] = true

			for i, keyword := range channel.Keywords {
				keyword = strings.ToLower(keyword)
				if seenKeywords[keyword] {
					return fmt.Errorf("duplicate keyword: %s", keyword)
				}
				seenKeywords[keyword] = true
				err := validate.VarWithValue(keyword, nil, "alphanumunicode")
				if err != nil {
					return fmt.Errorf("keywords must be alphanumeric got '%s'", keyword)
				}

				channel.Keywords[i] = keyword
			}
		}

		if len(interchange.Channels) == 0 {
			return fmt.Errorf("interchange must define at least one channel")
		}
	}

	return nil
}

// validate validates the passe din struct using our shared validator instance
func validateObject(obj interface{}) error {
	err := validate.Struct(obj)
	if err != nil {
		return err
	}
	return nil
}
