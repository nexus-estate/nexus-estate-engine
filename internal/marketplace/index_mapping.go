package marketplace

// ListingIndexDefinition is the canonical create-index request body for the
// marketplace listing index: the JSON to send to PUT /<index>. Its public listing
// fields mirror MarketplaceListingDocument; projection_state is reserved
// write-layer metadata for a future atomic revision/tombstone guard. Elasticsearch
// _id must be set from listing_id, because listing id is the document identity.
//
// Analysis is included because it changes how canonical text fields are searched.
// Shard and replica counts are deployment topology and belong in environment or
// index-lifecycle configuration, not this field contract. The custom listing_text
// analyzer folds case and diacritics so a query typed without accents still matches
// accented Vietnamese text. The built-in vietnamese analyzer is not used because
// its stopword removal drops short, high-intent tokens such as place names.
//
// dynamic: strict makes an unmapped field a write error instead of a silent
// mapping explosion. The Engine is the only writer and every document is validated
// before the write. projection_state is a separately mapped internal write shape;
// a future indexer must still reject unmapped fields in both objects.
//
// Index name, lifecycle policy and the write path belong to the indexing plan
// (ENGINE-02).
const ListingIndexDefinition = `{
  "settings": {
    "analysis": {
      "analyzer": {
        "listing_text": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": [
            "lowercase",
            "asciifolding"
          ]
        }
      }
    }
  },
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "listing_id": {
        "type": "keyword"
      },
      "property_id": {
        "type": "keyword"
      },
      "title": {
        "type": "text",
        "analyzer": "listing_text",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "slug": {
        "type": "keyword"
      },
      "description": {
        "type": "text",
        "analyzer": "listing_text"
      },
      "type": {
        "type": "keyword"
      },
      "purpose": {
        "type": "keyword"
      },
      "price": {
        "type": "double"
      },
      "area": {
        "type": "double"
      },
      "city": {
        "type": "text",
        "analyzer": "listing_text",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "district": {
        "type": "text",
        "analyzer": "listing_text",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "ward": {
        "type": "text",
        "analyzer": "listing_text",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "address": {
        "type": "text",
        "analyzer": "listing_text",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "location": {
        "type": "geo_point"
      },
      "media": {
        "properties": {
          "images": {
            "type": "keyword"
          },
          "cover_image": {
            "type": "keyword"
          }
        }
      },
      "published_at": {
        "type": "date",
        "format": "strict_date_optional_time||epoch_millis"
      },
      "updated_at": {
        "type": "date",
        "format": "strict_date_optional_time||epoch_millis"
      },
      "projection_state": {
        "type": "object",
        "dynamic": "strict",
        "properties": {
          "source_revision": {
            "type": "long"
          },
          "deleted": {
            "type": "boolean"
          }
        }
      }
    }
  }
}`
