package main

import "context"

func Set(ctx context.Context, key, value string) error {
	data, err := loadData(ctx)
	if err != nil {
		return err
	}
	data[key] = value
	err = saveData(ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func Get(ctx context.Context, key string) (string, error) {
	data, err := loadData(ctx)
	if err != nil {
		return "", nil
	}
	return data[key], nil
}

func Delete(ctx context.Context, key string) error {
	data, err := loadData(ctx)
	if err != nil {
		return err
	}
	delete(data, key)
	err = saveData(ctx, data)
	if err != nil {
		return err
	}
	return nil
}
