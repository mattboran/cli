package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cli/cli/api"
	"github.com/spf13/cobra"
)

func init() {
	prCmd.AddCommand(prAssignCmd)

	prAssignCmd.Flags().StringArrayP("login", "l", nil, "Specify a user login to be assigned")

	prCmd.MarkFlagRequired("login")
}

var prAssignCmd = &cobra.Command{
	Use:   "assign [{<number> | <url> | <branch>]",
	Short: "Assign a reviewer to a pull request.",
	Args:  cobra.MaximumNArgs(1),
	Long: `Assign a reviewer to either a specified pull request or the pull request associated with the current branch.

Examples:

	gh pr assign -l mattboran                     # assign mattboran as a reviewer for the current branch's pull request
	gh pr assign -l mattboran -l veeamd           # assign mattboran and veeamd as reviewers for the current branch's pull request
	gh pr assign 123 -l mattboran                 # assign mattboran as a reviewer for pull request 123
	`,
	RunE: prAssign,
}

func processAssignOpt(cmd *cobra.Command) ([]string, error) {
	logins, err := cmd.Flags().GetStringArray("login")
	if err != nil {
		return nil, err
	}
	if len(logins) == 0 {
		return nil, errors.New("need at least one username after --login")
	}
	return logins, err
}

func prAssign(cmd *cobra.Command, args []string) error {
	ctx := contextForCommand(cmd)
	apiClient, err := apiClientForContext(ctx)
	if err != nil {
		return err
	}

	baseRepo, err := determineBaseRepo(apiClient, cmd, ctx)
	if err != nil {
		return err
	}

	var prNum int
	branchWithOwner := ""

	if len(args) == 0 {
		prNum, branchWithOwner, err = prSelectorForCurrentBranch(ctx, baseRepo)
		if err != nil {
			return fmt.Errorf("could not query for pull request for current branch: %w", err)
		}
	} else {
		prArg, repo := prFromURL(args[0])
		if repo != nil {
			baseRepo = repo
		} else {
			prArg = strings.TrimPrefix(args[0], "#")
		}
		prNum, err = strconv.Atoi(prArg)
		if err != nil {
			return errors.New("could not parse pull request argument")
		}
	}

	var pr *api.PullRequest
	if prNum > 0 {
		pr, err = api.PullRequestByNumber(apiClient, baseRepo, prNum)
		if err != nil {
			return fmt.Errorf("could not find pull request: %w", err)
		}
	} else {
		pr, err = api.PullRequestForBranch(apiClient, baseRepo, "", branchWithOwner)
		if err != nil {
			return fmt.Errorf("could not find pull request: %w", err)
		}
		prNum = pr.Number
	}

	logins, err := processAssignOpt(cmd)
	if err != nil {
		return err
	}

	out := colorableOut(cmd)
	assignees := pr.Assignees.Nodes
	owner := pr.HeadRepositoryOwner.Login

	// TODO: - Get the team from the query
	teamName := "mobile"
	teamMembers, err := api.MobileTeamMembers(apiClient, owner, teamName)
	if err != nil {
		return err
	}

	// Header info
	// TODO: - Add PR info here
	if len(assignees) == 0 {
		fmt.Fprintf(out, "No reviewers currently assigned.")
	} else {
		var logins []string
		for _, member := range assignees {
			logins = append(logins, member.Login)
		}
		fmt.Fprintf(out, "Currently assigned: (%s)\n", strings.Join(logins, ", "))
	}

	var assignableIds []string
	var notFoundLogins []string
	teamMemberToIDMap := assignableMap(teamMembers)
	for _, login := range logins {
		if id, found := teamMemberToIDMap[login]; found {
			assignableIds = append(assignableIds, id)
		} else {
			notFoundLogins = append(notFoundLogins, login)
		}
	}
	if notFoundLogins != nil {
		logins := strings.Join(notFoundLogins, ", ")
		errorMessage := fmt.Sprintf("Could not find logins (%s) in team %s", logins, teamName)
		return errors.New(errorMessage)
	}

	for index, ID := range assignableIds {
		fmt.Fprintf(out, "Assigning user %s (ID %s) to assignable ID %s\n", logins[index], ID, pr.ID)
	}

	return nil
}

func assignableMap(teamMembers []api.TeamMember) map[string]string {
	result := make(map[string]string)
	for _, member := range teamMembers {
		result[member.Login] = member.ID
	}
	return result
}
