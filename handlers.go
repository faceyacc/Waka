package main

import (
	"context"
)

// Handlers
func Get(ctx context.Context, key string) (string, error) {
	data, err := loadData(ctx)
	if err != nil {
		return "", err
	}

	return data[key], nil
}

func Delete(ctx context.Context, key string) error {

	// Load data
	data, err := loadData(ctx)
	if err != nil {
		return err
	}

	delete(data, key)

	// Save data
	err = saveData(ctx, data)
	if err != nil {
		return err
	}

	return nil
}

func Set(ctx context.Context, key string, value string) error {

	// load data
	data, err := loadData(ctx)
	if err != nil {
		return err
	}

	data[key] = value

	// Save data
	err = saveData(ctx, data)
	if err != nil {
		return err
	}
	return nil
}
