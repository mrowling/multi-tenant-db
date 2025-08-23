package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_Authentication(t *testing.T) {
	// Save original env vars
	originalUser := os.Getenv("AUTH_USERNAME")
	originalPass := os.Getenv("AUTH_PASSWORD")
	defer func() {
		os.Setenv("AUTH_USERNAME", originalUser)
		os.Setenv("AUTH_PASSWORD", originalPass)
	}()

	tests := []struct {
		name         string
		username     string
		password     string
		expectedUser string
		expectedPass string
		shouldHaveAuth bool
	}{
		{
			name:           "both username and password",
			username:       "testuser",
			password:       "testpass",
			expectedUser:   "testuser",
			expectedPass:   "testpass",
			shouldHaveAuth: true,
		},
		{
			name:           "only password - should default username",
			username:       "",
			password:       "testpass",
			expectedUser:   "root",
			expectedPass:   "testpass",
			shouldHaveAuth: true,
		},
		{
			name:           "only username",
			username:       "testuser",
			password:       "",
			expectedUser:   "testuser",
			expectedPass:   "",
			shouldHaveAuth: true,
		},
		{
			name:           "neither - no auth config",
			username:       "",
			password:       "",
			shouldHaveAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("AUTH_USERNAME")
			os.Unsetenv("AUTH_PASSWORD")
			
			// Set test values
			if tt.username != "" {
				os.Setenv("AUTH_USERNAME", tt.username)
			}
			if tt.password != "" {
				os.Setenv("AUTH_PASSWORD", tt.password)
			}

			cfg := NewConfig()
			err := cfg.LoadFromEnv()
			if err != nil {
				t.Fatalf("LoadFromEnv failed: %v", err)
			}

			if tt.shouldHaveAuth {
				if cfg.Auth == nil {
					t.Fatal("Expected auth configuration")
				}
				if cfg.Auth.Username != tt.expectedUser {
					t.Errorf("Expected username %s, got %s", tt.expectedUser, cfg.Auth.Username)
				}
				if cfg.Auth.Password != tt.expectedPass {
					t.Errorf("Expected password %s, got %s", tt.expectedPass, cfg.Auth.Password)
				}
			} else {
				if cfg.Auth != nil {
					t.Error("Expected no auth configuration")
				}
			}
		})
	}
}

func TestAuthConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   AuthConfig
		hasError bool
	}{
		{
			name: "valid with password",
			config: AuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			hasError: false,
		},
		{
			name: "valid without password",
			config: AuthConfig{
				Username: "testuser",
				Password: "",
			},
			hasError: false,
		},
		{
			name: "invalid - no username",
			config: AuthConfig{
				Username: "",
				Password: "testpass",
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.hasError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConfigValidateWithAuth(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		hasError bool
	}{
		{
			name: "valid config with auth",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 3306,
				Auth: &AuthConfig{
					Username: "testuser",
					Password: "testpass",
				},
			},
			hasError: false,
		},
		{
			name: "invalid config with invalid auth",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 3306,
				Auth: &AuthConfig{
					Username: "", // Invalid - empty username
					Password: "testpass",
				},
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.hasError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
