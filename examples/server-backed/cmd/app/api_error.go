package main

type apiError string

func (err apiError) Error() string {
	return string(err)
}
