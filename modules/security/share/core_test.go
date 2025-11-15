/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	"fmt"
	"infini.sh/framework/core/orm"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharingPermissionLevels(t *testing.T) {
	// Test permission hierarchy
	assert.True(t, Owner > Share)
	assert.True(t, Share > Edit)
	assert.True(t, Edit > Comment)
	assert.True(t, Comment > View)
	assert.True(t, View > 0)
}

func TestPermissionToString(t *testing.T) {
	tests := []struct {
		permission SharingPermission
		expected   string
	}{
		{View, "view"},
		{Comment, "comment"},
		{Edit, "edit"},
		{Share, "share"},
		{Owner, "owner"},
		{SharingPermission(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := permissionToString(tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSharingPermission(t *testing.T) {
	tests := []struct {
		input    string
		expected SharingPermission
		hasError bool
	}{
		{"view", View, false},
		{"comment", Comment, false},
		{"edit", Edit, false},
		{"share", Share, false},
		{"owner", Owner, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseSharingPermission(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"/", []string{}},
		{"/documents", []string{"documents"}},
		{"/documents/file.pdf", []string{"documents", "file.pdf"}},
		{"/documents/2023/reports", []string{"documents", "2023", "reports"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{}, "/"},
		{[]string{"documents"}, "/documents"},
		{[]string{"documents", "file.pdf"}, "/documents/file.pdf"},
		{[]string{"documents", "2023", "reports"}, "/documents/2023/reports"}}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := joinPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSecureToken(t *testing.T) {
	token1, err := generateSecureToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token1)
	assert.Len(t, token1, 32) // 16 bytes = 32 hex characters

	token2, err := generateSecureToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2, "Tokens should be unique")
}

func TestPasswordHashing(t *testing.T) {
	password := "testPassword123"

	hash1 := hashPassword(password)
	assert.NotEmpty(t, hash1)
	assert.NotEqual(t, password, hash1)

	// Same password should produce same hash
	hash2 := hashPassword(password)
	assert.Equal(t, hash1, hash2)

	// Different password should produce different hash
	hash3 := hashPassword("differentPassword")
	assert.NotEqual(t, hash1, hash3)

	// Verify password
	assert.True(t, verifyPassword(password, hash1))
	assert.False(t, verifyPassword("wrongPassword", hash1))
}

func TestLightweightPermissionChecker(t *testing.T) {
	checker := NewLightweightPermissionChecker()

	// Test single permission
	checker.AddPermission("resource1", Edit)
	assert.True(t, checker.HasPermission("resource1", View))
	assert.True(t, checker.HasPermission("resource1", Edit))
	assert.False(t, checker.HasPermission("resource1", Share))
	assert.False(t, checker.HasPermission("nonexistent", View))

	// Test multiple permissions
	checker.AddPermission("resource2", Owner)
	assert.True(t, checker.HasAnyPermission("resource2", View, Edit, Share))
	assert.True(t, checker.HasAnyPermission("resource2", Owner))
	assert.False(t, checker.HasAnyPermission("resource2", SharingPermission(99)))

	// Test non-existent resource
	assert.False(t, checker.HasPermission("unknown", View))
	assert.False(t, checker.HasAnyPermission("unknown", View, Edit))
}

func TestPermissionFilter(t *testing.T) {
	service := NewSharingService()

	tests := []struct {
		name     string
		filter   PermissionFilter
		validate func(t *testing.T, qb *orm.QueryBuilder)
	}{
		{
			name: "Basic user filter",
			filter: PermissionFilter{
				UserID: "user123",
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have user-specific filters
			},
		},
		{
			name: "With groups",
			filter: PermissionFilter{
				UserID:     "user123",
				UserGroups: []string{"group1", "group2"},
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have group filters
			},
		},
		{
			name: "With resource IDs",
			filter: PermissionFilter{
				UserID:      "user123",
				ResourceIDs: []string{"res1", "res2"},
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have resource ID filters
			},
		},
		{
			name: "With path filtering",
			filter: PermissionFilter{
				UserID:       "user123",
				ResourcePath: "/documents/2023",
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have path-based filters
			},
		},
		{
			name: "With permission level",
			filter: PermissionFilter{
				UserID:     "user123",
				Permission: Edit,
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have permission level filter
			},
		},
		{
			name: "Include public shares",
			filter: PermissionFilter{
				UserID:        "user123",
				IncludePublic: true,
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should include public share filters
			},
		},
		{
			name: "Complete filter",
			filter: PermissionFilter{
				UserID:        "user123",
				UserGroups:    []string{"group1", "group2"},
				ResourceIDs:   []string{"res1", "res2"},
				ResourcePath:  "/documents",
				Permission:    Edit,
				IncludePublic: true,
			},
			validate: func(t *testing.T, qb *orm.QueryBuilder) {
				assert.NotNil(t, qb)
				// Should have all filter types
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb := service.BuildPermissionFilter(tt.filter)
			tt.validate(t, qb)
		})
	}
}

func TestPermissionInheritance(t *testing.T) {
	// Test that higher permissions override lower ones
	permissions := []SharingPermission{View, Comment, Edit, Share, Owner}

	for i, perm := range permissions {
		t.Run(permissionToString(perm), func(t *testing.T) {
			// This permission should grant access to all lower permissions
			for j := 0; j <= i; j++ {
				assert.True(t, perm >= permissions[j])
			}
			// But not to higher permissions
			for j := i + 1; j < len(permissions); j++ {
				assert.False(t, perm >= permissions[j])
			}
		})
	}
}

// Integration tests (these would need a test database setup)
func TestSharingService_Integration(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	service := NewSharingService()

	// Test creating and validating share links
	t.Run("ShareLinkLifecycle", func(t *testing.T) {
		link, err := service.CreateShareLink(
			"test-resource",
			"file",
			"/test/path",
			"test-user",
			View,
			nil,
			"",
		)
		require.NoError(t, err)
		require.NotNil(t, link)

		// Validate the link
		validatedLink, err := service.ValidateShareLink(link.Token, "")
		require.NoError(t, err)
		assert.Equal(t, link.Token, validatedLink.Token)
		assert.Equal(t, int64(1), validatedLink.AccessCount)
	})

	// Test permission checking
	t.Run("PermissionChecking", func(t *testing.T) {
		permission, err := service.GetUserExplicitEffectivePermission("test-user", NewResourceEntity("file", "test-resource", "/test/path"))
		require.NoError(t, err)
		assert.Equal(t, View, permission)
	})

	// Test accessible resources
	t.Run("AccessibleResources", func(t *testing.T) {
		resources, err := service.GetUserAccessibleResources("test-user", View)
		require.NoError(t, err)
		assert.Contains(t, resources, "test-resource")
	})
}

// Test edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	t.Run("EmptyPaths", func(t *testing.T) {
		assert.Equal(t, []string{}, splitPath(""))
		assert.Equal(t, []string{}, splitPath("/"))
		assert.Equal(t, "/", joinPath([]string{}))
	})

	t.Run("MalformedPaths", func(t *testing.T) {
		assert.Equal(t, []string{"docs"}, splitPath("docs"))
		assert.Equal(t, []string{"docs"}, splitPath("/docs"))
		assert.Equal(t, []string{"docs", "file"}, splitPath("docs/file"))
		assert.Equal(t, []string{"docs", "file"}, splitPath("/docs/file"))
	})

	t.Run("SpecialCharactersInPaths", func(t *testing.T) {
		assert.Equal(t, []string{"user@domain.com", "file.txt"}, splitPath("/user@domain.com/file.txt"))
		assert.Equal(t, []string{"folder-with-dash", "file_name"}, splitPath("/folder-with-dash/file_name"))
		assert.Equal(t, []string{"folder.with.dots", "file.pdf"}, splitPath("/folder.with.dots/file.pdf"))
	})

	t.Run("UnicodePaths", func(t *testing.T) {
		assert.Equal(t, []string{"文档", "报告.pdf"}, splitPath("/文档/报告.pdf"))
		assert.Equal(t, []string{"共享", "文件夹", "文件.txt"}, splitPath("/共享/文件夹/文件.txt"))
	})
}

// Test concurrent access (thread safety)
func TestConcurrentAccess(t *testing.T) {
	checker := NewLightweightPermissionChecker()

	// Add permissions concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(i int) {
			resourceID := fmt.Sprintf("resource%d", i)
			permission := SharingPermission(i%5 + 1) // View to Owner
			checker.AddPermission(resourceID, permission)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify all permissions were added
	for i := 0; i < 100; i++ {
		resourceID := fmt.Sprintf("resource%d", i)
		expectedPerm := SharingPermission(i%5 + 1)
		actual, ok := checker.GetPermission(resourceID)
		assert.Equal(t, true, ok)
		assert.Equal(t, expectedPerm, actual)
	}
}

// Test memory usage patterns
func TestMemoryEfficiency(t *testing.T) {
	checker := NewLightweightPermissionChecker()

	// Add many permissions
	for i := 0; i < 10000; i++ {
		resourceID := fmt.Sprintf("resource_%d", i)
		checker.AddPermission(resourceID, View)
	}

	// Verify we can still check permissions efficiently
	start := time.Now()
	hasAccess := checker.HasPermission("resource_5000", View)
	duration := time.Since(start)

	assert.True(t, hasAccess)
	assert.Less(t, duration, time.Millisecond, "Permission check should be very fast")
}

// Test filter combinations
func TestComplexFilterScenarios(t *testing.T) {
	service := NewSharingService()

	t.Run("MultipleResourceIDs", func(t *testing.T) {
		filter := PermissionFilter{
			UserID:      "user123",
			ResourceIDs: []string{"res1", "res2", "res3", "res4", "res5"},
		}
		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb)
	})

	t.Run("DeepPathHierarchy", func(t *testing.T) {
		filter := PermissionFilter{
			UserID:       "user123",
			ResourcePath: "/very/deep/path/structure/with/many/levels/folder",
		}
		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb)
	})

	t.Run("MaximumPermission", func(t *testing.T) {
		filter := PermissionFilter{
			UserID:     "user123",
			Permission: Owner,
		}
		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb)
	})
}

// Test error handling and validation
func TestErrorHandling(t *testing.T) {
	service := NewSharingService()

	t.Run("InvalidPermission", func(t *testing.T) {
		_, err := parseSharingPermission("invalid_permission")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid permission")
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		filter := PermissionFilter{
			UserID: "",
		}
		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb) // Should still work, just no user filter
	})

	t.Run("EmptyResourceIDs", func(t *testing.T) {
		filter := PermissionFilter{
			UserID:      "user123",
			ResourceIDs: []string{},
		}
		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb)
	})
}

// Test compatibility with existing systems
func TestCompatibility(t *testing.T) {
	t.Run("PermissionValues", func(t *testing.T) {
		// Ensure our permission values are compatible with expected ranges
		assert.GreaterOrEqual(t, int(View), 1)
		assert.LessOrEqual(t, int(Owner), 31) // Using bit flags, max would be 31

		// Test bit flag operations
		combined := View | Edit | Share
		assert.True(t, combined&View != 0)
		assert.True(t, combined&Edit != 0)
		assert.True(t, combined&Share != 0)
		assert.False(t, combined&Owner != 0)
	})

	t.Run("StringConversions", func(t *testing.T) {
		// Test round-trip conversions
		permissions := []SharingPermission{View, Comment, Edit, Share, Owner}

		for _, perm := range permissions {
			str := permissionToString(perm)
			parsed, err := parseSharingPermission(str)
			assert.NoError(t, err)
			assert.Equal(t, perm, parsed, "Round-trip conversion failed for %s", str)
		}
	})
}

// Test documentation examples
func TestDocumentationExamples(t *testing.T) {
	t.Run("PermissionFilterExample", func(t *testing.T) {
		service := NewSharingService()

		// Example: Filter for a specific user with path-based permissions
		filter := PermissionFilter{
			UserID:        "john.doe@company.com",
			ResourcePath:  "/projects/alpha",
			Permission:    Edit,
			IncludePublic: true,
		}

		qb := service.BuildPermissionFilter(filter)
		assert.NotNil(t, qb)

		// Example: Complex filter with multiple criteria
		complexFilter := PermissionFilter{
			UserID:        "jane.smith",
			UserGroups:    []string{"engineering", "managers"},
			ResourceIDs:   []string{"doc1", "doc2", "doc3"},
			ResourcePath:  "/shared/engineering",
			Permission:    Share,
			IncludePublic: true,
		}

		complexQb := service.BuildPermissionFilter(complexFilter)
		assert.NotNil(t, complexQb)
	})

	t.Run("LightweightCheckerExample", func(t *testing.T) {
		checker := NewLightweightPermissionChecker()

		// Example: UI permission checking
		checker.AddPermission("file123", Edit)
		checker.AddPermission("file456", View)
		checker.AddPermission("file789", Owner)

		// Check if user can edit a specific file
		canEdit := checker.HasPermission("file123", Edit)
		assert.True(t, canEdit)

		// Check if user has any of several permissions
		hasAnyPerm := checker.HasAnyPermission("file789", Share, Owner)
		assert.True(t, hasAnyPerm)
	})
}


// Test security considerations
func TestSecurityConsiderations(t *testing.T) {
	t.Run("TokenUniqueness", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			token, err := generateSecureToken()
			require.NoError(t, err)

			// Each token should be unique
			assert.False(t, tokens[token], "Duplicate token generated")
			tokens[token] = true
		}
	})

	t.Run("PasswordHashing", func(t *testing.T) {
		password := "weakPassword123"
		hash := hashPassword(password)

		// Hash should not reveal original password
		assert.NotEqual(t, password, hash)
		assert.NotContains(t, hash, password)

		// Should be able to verify correct password
		assert.True(t, verifyPassword(password, hash))

		// Should reject wrong password
		assert.False(t, verifyPassword("wrongPassword", hash))
	})

	t.Run("PermissionEscalation", func(t *testing.T) {
		checker := NewLightweightPermissionChecker()
		checker.AddPermission("resource", View)

		// Should not be able to escalate permissions
		assert.False(t, checker.HasPermission("resource", Edit))
		assert.False(t, checker.HasPermission("resource", Share))
		assert.False(t, checker.HasPermission("resource", Owner))
	})
}
