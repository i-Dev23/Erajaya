package domain

import "context"

// model domain generate token request business logic
type GeneratedTokenRequestDomain struct {
	ClientID     string
	ClientSecret string
	Timestamp    string
}

// interface usecase token business logic
// * pointer because we don't want to pass by value, we want to pass by reference
type TokenUsecase interface {
	GenerateAndStoreToken(ctx context.Context, request *GeneratedTokenRequestDomain) (string, error)
	ValidateToken(ctx context.Context, tokenValue string) error
}
