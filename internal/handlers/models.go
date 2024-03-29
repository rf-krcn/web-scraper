package handlers

import (
	"errors"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type baseQuote struct {
	QuoteText string   `json:"text"`
	Author    string   `json:"author"`
	Tags      []string `json:"tags"`
}

type Quote struct {
	gorm.Model
	QuoteText string `json:"text"`
	Author    string `json:"author"`
	Tags      []Tag  `gorm:"many2many:quote_tags;"`
}

type Tag struct {
	gorm.Model
	Name   string  `json:"name"`
	Quotes []Quote `gorm:"many2many:quote_tags;"`
}

var db *gorm.DB

func New(dbPool *gorm.DB) error {
	db = dbPool
	return db.AutoMigrate(&Quote{}, &Tag{})
}

func GetByAuthorDB(author string) ([]baseQuote, error) {

	var quotes []Quote
	result := db.Preload("Tags").Where("author = ?", author).Find(&quotes)
	if result.Error != nil {
		return nil, result.Error
	}

	baseQuotes := basingQuote(quotes)
	return baseQuotes, nil
}

func GetByTagDB(tag string) ([]baseQuote, error) {
	var quotes []Quote
	result := db.Preload("Tags").Joins("JOIN quote_tags ON quotes.id = quote_tags.quote_id").
		Joins("JOIN tags ON tags.id = quote_tags.tag_id").
		Where("tags.name = ?", tag).Find(&quotes)
	if result.Error != nil {
		return nil, result.Error
	}

	baseQuotes := basingQuote(quotes)
	return baseQuotes, nil
}

func GetAllDB() ([]baseQuote, error) {

	var quotes []Quote
	result := db.Preload("Tags").Find(&quotes)
	if result.Error != nil {
		return nil, result.Error
	}

	baseQuotes := basingQuote(quotes)
	return baseQuotes, nil
}

func GetAllTagsDB() ([]string, error) {

	var tags []Tag
	result := db.Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}
	var baseTags []string
	for _, tag := range tags {
		baseTags = append(baseTags, tag.Name)
	}

	return baseTags, nil

}

func DatabaseIsEmpty() (bool, error) {
	var count int64
	result := db.Model(&Quote{}).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}

	return count == 0, nil
}

func AddQuoteWithTags(quote Quote, tagNames []string) error {
	tx := db.Begin()

	if err := tx.Create(&quote).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, name := range tagNames {
		var tag Tag
		if err := tx.Where("name = ?", name).First(&tag).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return err
			}
			tag.Name = name
			if err := tx.Create(&tag).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Model(&quote).Association("Tags").Append(&tag); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func basingQuote(quotes []Quote) []baseQuote {

	var baseQuotes []baseQuote

	for _, quote := range quotes {

		var baseTags []string
		for _, tag := range quote.Tags {
			baseTags = append(baseTags, tag.Name)
		}
		baseQuote := baseQuote{
			QuoteText: quote.QuoteText,
			Author:    quote.Author,
			Tags:      baseTags,
		}
		baseQuotes = append(baseQuotes, baseQuote)

	}
	return baseQuotes
}

func ConnectToDB(dsn string) *gorm.DB {
	var counts int64
	for {
		connection, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Println("Postgres not yet ready ...")
			counts++
		} else {
			log.Println("Connected to Postgres!")
			return connection
		}

		if counts > 10 {
			log.Println(err)
			return nil
		}

		log.Println("Backing off for two seconds....")
		time.Sleep(2 * time.Second)
		continue
	}
}
