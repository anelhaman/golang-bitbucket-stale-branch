package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
)

// BitbucketClient represents a client to interact with the Bitbucket API using Resty
type BitbucketClient struct {
	BaseURL   string
	AuthToken string
	Client    *resty.Client
	Workspace string
}

// NewBitbucketClient creates a new BitbucketClient instance
func NewBitbucketClient(workspace, authToken string) *BitbucketClient {
	client := resty.New()

	return &BitbucketClient{
		BaseURL:   "https://api.bitbucket.org/2.0",
		AuthToken: authToken,
		Client:    client,
		Workspace: workspace,
	}
}

// FetchRepositories fetches all repositories from the Bitbucket workspace
func (b *BitbucketClient) FetchRepositories() ([]map[string]interface{}, error) {
	repos := []map[string]interface{}{}
	resp, err := b.Client.R().
		SetAuthToken(b.AuthToken).
		SetResult(&repos).
		Get(fmt.Sprintf("%s/repositories/%s", b.BaseURL, b.Workspace))

	if err != nil {
		return nil, err
	}

	// Check for successful response
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to fetch repositories: %s", resp.Status())
	}

	return repos, nil
}

// FetchBranches fetches branches for a specific repository
func (b *BitbucketClient) FetchBranches(repoSlug string) ([]map[string]interface{}, error) {
	branches := []map[string]interface{}{}
	resp, err := b.Client.R().
		SetAuthToken(b.AuthToken).
		SetResult(&branches).
		Get(fmt.Sprintf("%s/repositories/%s/%s/refs/branches", b.BaseURL, b.Workspace, repoSlug))

	if err != nil {
		return nil, err
	}

	// Check for successful response
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to fetch branches for repo %s: %s", repoSlug, resp.Status())
	}

	return branches, nil
}

// CheckIfStale checks if a branch is stale based on the last commit date (3 months threshold)
func (b *BitbucketClient) CheckIfStale(branch map[string]interface{}, threshold time.Duration) (bool, time.Duration) {
	commitDateStr := branch["target"].(map[string]interface{})["date"].(string)
	commitDate, err := time.Parse(time.RFC3339, commitDateStr)
	if err != nil {
		return false, 0
	}
	daysSinceCommit := time.Since(commitDate).Hours() / 24
	if daysSinceCommit > threshold.Hours()/24 {
		return true, time.Duration(daysSinceCommit) * time.Hour
	}
	return false, 0
}

// DeleteBranch deletes a specific branch in a repository, excluding protected branches
func (b *BitbucketClient) DeleteBranch(repoSlug, branchName string) error {
	// List of protected branch names that should not be deleted
	protectedBranches := []string{"main", "master", "develop"}

	// Check if the branch is protected
	for _, protectedBranch := range protectedBranches {
		if branchName == protectedBranch {
			fmt.Printf("Branch %s is protected and cannot be deleted.\n", branchName)
			return nil
		}
	}

	// Proceed with deletion if the branch is not protected
	resp, err := b.Client.R().
		SetAuthToken(b.AuthToken).
		Delete(fmt.Sprintf("%s/repositories/%s/%s/refs/branches/%s", b.BaseURL, b.Workspace, repoSlug, branchName))

	if err != nil {
		return err
	}

	// Check for successful response
	if resp.StatusCode() != 204 {
		return fmt.Errorf("failed to delete branch %s: %s", branchName, resp.Status())
	}

	fmt.Printf("Branch %s deleted in repository %s\n", branchName, repoSlug)
	return nil
}

// ListStaleBranches lists all stale branches across all repositories in the workspace
// If delete is true, it will also delete the stale branches, excluding protected branches
func (b *BitbucketClient) ListStaleBranches(threshold time.Duration, delete bool) {
	repos, err := b.FetchRepositories()
	if err != nil {
		fmt.Printf("Error fetching repositories: %v\n", err)
		return
	}

	for _, repo := range repos {
		repoSlug := repo["slug"].(string)
		fmt.Printf("Checking branches for repository: %s\n", repoSlug)

		branches, err := b.FetchBranches(repoSlug)
		if err != nil {
			fmt.Printf("Error fetching branches for repo %s: %v\n", repoSlug, err)
			continue
		}

		for _, branch := range branches {
			isStale, nonInteractDays := b.CheckIfStale(branch, threshold)
			if isStale {
				fmt.Printf("Stale branch found: %s in repo %s, non-interacted for approximate %.0f days\n", branch["name"], repoSlug, nonInteractDays.Hours()/24)
				if delete {
					// if err := b.DeleteBranch(repoSlug, branch["name"].(string)); err != nil {
					// 	fmt.Printf("Error deleting branch %s: %v\n", branch["name"], err)
					// }
					fmt.Printf("Blaaa")
				}
			}
		}
	}
}

func main() {

	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		// return
	}

	// Read the Bitbucket token and workspace from environment variables
	authToken := os.Getenv("BITBUCKET_TOKEN")
	if authToken == "" {
		fmt.Println("Error: BITBUCKET_TOKEN environment variable is not set")
		return
	}

	workspace := os.Getenv("BITBUCKET_WORKSPACE")
	if workspace == "" {
		fmt.Println("Error: BITBUCKET_WORKSPACE environment variable is not set")
		return
	}

	// Initialize the Bitbucket client with workspace and auth token
	client := NewBitbucketClient(workspace, authToken)

	// Define the threshold for stale branches (e.g., 3 months)
	threshold := 3 * 30 * 24 * time.Hour // 3 months (in hours)

	// Set delete flag to true to delete stale branches
	delete := false // Change to true to enable deletion

	// List stale branches across all repositories in the workspace, with delete flag
	client.ListStaleBranches(threshold, delete)
}
