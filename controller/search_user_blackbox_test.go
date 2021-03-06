package controller_test

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/fabric8-services/fabric8-auth/account"
	"github.com/fabric8-services/fabric8-auth/app"
	"github.com/fabric8-services/fabric8-auth/app/test"
	"github.com/fabric8-services/fabric8-auth/application"
	. "github.com/fabric8-services/fabric8-auth/controller"
	"github.com/fabric8-services/fabric8-auth/gormapplication"
	"github.com/fabric8-services/fabric8-auth/gormtestsupport"
	"github.com/fabric8-services/fabric8-auth/resource"
	"github.com/goadesign/goa"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRunSearchUser(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TestSearchUserSearch{DBTestSuite: gormtestsupport.NewDBTestSuite()})
}

type TestSearchUserSearch struct {
	gormtestsupport.DBTestSuite
	svc        *goa.Service
	controller *SearchController
}

func (s *TestSearchUserSearch) SetupSuite() {
	s.DBTestSuite.SetupSuite()
	s.svc = goa.New("test")
	s.controller = NewSearchController(s.svc, s.Application, s.Configuration)
}

type userSearchTestArgs struct {
	pageOffset *string
	pageLimit  *int
	q          string
}

type userSearchTestExpect func(*testing.T, okScenarioUserSearchTest, *app.UserList)
type userSearchTestExpects []userSearchTestExpect

type okScenarioUserSearchTest struct {
	name                  string
	userSearchTestArgs    userSearchTestArgs
	userSearchTestExpects userSearchTestExpects
}

func (s *TestSearchUserSearch) TestUsersSearchOK() {

	idents := s.createTestData()
	defer s.cleanTestData(idents)

	tests := []okScenarioUserSearchTest{
		{"Without A-Z ,a-z or 0-9", userSearchTestArgs{s.offset(0), s.limit(10), "."}, userSearchTestExpects{s.totalCount(0)}},
		{"Without A-Z ,a-z or 0-9", userSearchTestArgs{s.offset(0), s.limit(10), ".@"}, userSearchTestExpects{s.totalCount(0)}},
		{"Without A-Z ,a-z or 0-9", userSearchTestArgs{s.offset(0), s.limit(10), "a@"}, userSearchTestExpects{s.totalCountAtLeast(1)}},
		{"With lowercase fullname query", userSearchTestArgs{s.offset(0), s.limit(10), "x_test_ab"}, userSearchTestExpects{s.totalCountAtLeast(3)}},
		{"With uppercase fullname query", userSearchTestArgs{s.offset(0), s.limit(10), "X_TEST_AB"}, userSearchTestExpects{s.totalCountAtLeast(3)}},
		{"With uppercase email query", userSearchTestArgs{s.offset(0), s.limit(10), "EMAIL_X_TEST_AB"}, userSearchTestExpects{s.totalCountAtLeast(1)}},
		{"With lowercase email query", userSearchTestArgs{s.offset(0), s.limit(10), "email_x_test_ab"}, userSearchTestExpects{s.totalCountAtLeast(1)}},
		{"With username query", userSearchTestArgs{s.offset(0), s.limit(10), "x_test_c"}, userSearchTestExpects{s.totalCountAtLeast(3)}},
		{"with special chars", userSearchTestArgs{s.offset(0), s.limit(10), "&:\n!#%?*"}, userSearchTestExpects{s.totalCount(0)}},
		{"with multi page", userSearchTestArgs{s.offset(0), s.limit(10), "TEST"}, userSearchTestExpects{s.hasLinks("Next")}},
		{"with last page", userSearchTestArgs{s.offset(len(idents) - 1), s.limit(10), "TEST"}, userSearchTestExpects{s.hasNoLinks("Next"), s.hasLinks("Prev")}},
		{"with different values", userSearchTestArgs{s.offset(0), s.limit(10), "TEST"}, userSearchTestExpects{s.differentValues()}},
		{"With offset exceeded the max limit total count", userSearchTestArgs{s.offset(s.Configuration.GetMaxUsersListLimit() + 1), s.limit(1), "TEST_"}, userSearchTestExpects{s.totalCount(s.Configuration.GetMaxUsersListLimit())}},
		{"With offset exceeded the max limit result size", userSearchTestArgs{s.offset(s.Configuration.GetMaxUsersListLimit() + 1), s.limit(1), "TEST_"}, userSearchTestExpects{s.resultLen(0)}},
		{"With offset + limit exceeded the max limit total count", userSearchTestArgs{s.offset(0), s.limit(s.Configuration.GetMaxUsersListLimit() + 1), "TEST_"}, userSearchTestExpects{s.totalCount(s.Configuration.GetMaxUsersListLimit())}},
		{"With offset + limit exceeded the max limit result size", userSearchTestArgs{s.offset(0), s.limit(s.Configuration.GetMaxUsersListLimit() + 1), "TEST_"}, userSearchTestExpects{s.resultLen(s.Configuration.GetMaxUsersListLimit())}},
		{"Within the max limit total count", userSearchTestArgs{s.offset(10), s.limit(5), "TEST_"}, userSearchTestExpects{s.totalCount(s.Configuration.GetMaxUsersListLimit())}},
		{"Within the max limit result size", userSearchTestArgs{s.offset(10), s.limit(5), "TEST_"}, userSearchTestExpects{s.resultLen(5)}},
	}

	for _, tt := range tests {
		_, result := test.UsersSearchOK(s.T(), context.Background(), s.svc, s.controller, tt.userSearchTestArgs.pageLimit, tt.userSearchTestArgs.pageOffset, tt.userSearchTestArgs.q)
		for _, userSearchTestExpect := range tt.userSearchTestExpects {
			userSearchTestExpect(s.T(), tt, result)
		}
	}
}

func (s *TestSearchUserSearch) TestUsersSearchBadRequest() {
	t := s.T()
	tests := []struct {
		name               string
		userSearchTestArgs userSearchTestArgs
	}{
		{"with empty query", userSearchTestArgs{s.offset(0), s.limit(10), ""}},
	}

	for _, tt := range tests {
		test.UsersSearchBadRequest(t, context.Background(), s.svc, s.controller, tt.userSearchTestArgs.pageLimit, tt.userSearchTestArgs.pageOffset, tt.userSearchTestArgs.q)
	}
}

func (s *TestSearchUserSearch) createTestData() []account.Identity {
	names := []string{"X_TEST_A", "X_TEST_AB", "X_TEST_B", "X_TEST_C"}
	emails := []string{"email_x_test_ab@redhat.org", "email_x_test_a@redhat.org", "email_x_test_c@redhat.org", "email_x_test_b@redhat.org"}
	usernames := []string{"x_test_b", "x_test_c", "x_test_a", "x_test_ab"}
	for i := 0; i < s.Configuration.GetMaxUsersListLimit(); i++ {
		names = append(names, "TEST_"+strconv.Itoa(i))
		emails = append(emails, "myemail"+strconv.Itoa(i))
		usernames = append(usernames, "myusernames"+strconv.Itoa(i))
	}

	idents := []account.Identity{}

	err := application.Transactional(s.Application, func(app application.Application) error {
		for i, name := range names {

			user := account.User{
				FullName: name,
				ImageURL: "http://example.org/" + name + ".png",
				Email:    emails[i],
				Cluster:  "default Cluster",
			}
			err := app.Users().Create(context.Background(), &user)
			require.Nil(s.T(), err)

			ident := account.Identity{
				User:         user,
				Username:     usernames[i] + uuid.NewV4().String(),
				ProviderType: "kc",
			}
			err = app.Identities().Create(context.Background(), &ident)
			require.Nil(s.T(), err)

			idents = append(idents, ident)
		}
		return nil
	})
	require.Nil(s.T(), err)
	return idents
}

func (s *TestSearchUserSearch) cleanTestData(idents []account.Identity) {
	err := application.Transactional(s.Application, func(app application.Application) error {
		db := app.(*gormapplication.GormTransaction).DB()
		db = db.Unscoped()
		for _, ident := range idents {
			db.Delete(ident)
			db.Delete(&account.User{}, "id = ?", ident.User.ID)
		}
		return nil
	})
	require.Nil(s.T(), err)
}

func (s *TestSearchUserSearch) totalCount(count int) userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		if got := result.Meta.TotalCount; got != count {
			t.Errorf("%s got = %v, want %v", scenario.name, got, count)
		}
	}
}

func (s *TestSearchUserSearch) totalCountAtLeast(count int) userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		got := result.Meta.TotalCount
		if !(got >= count) {
			t.Errorf("%s got %v, wanted at least %v", scenario.name, got, count)
		}
	}
}

func (s *TestSearchUserSearch) resultLen(length int) userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		if length != len(result.Data) {
			t.Errorf("%s got %v, wanted %v", scenario.name, len(result.Data), length)
		}
	}
}

func (s *TestSearchUserSearch) hasLinks(linkNames ...string) userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		for _, linkName := range linkNames {
			link := linkName
			if reflect.Indirect(reflect.ValueOf(result.Links)).FieldByName(link).IsNil() {
				t.Errorf("%s got empty link, wanted %s", scenario.name, link)
			}
		}
	}
}

func (s *TestSearchUserSearch) hasNoLinks(linkNames ...string) userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		for _, linkName := range linkNames {
			if !reflect.Indirect(reflect.ValueOf(result.Links)).FieldByName(linkName).IsNil() {
				t.Errorf("%s got link, wanted empty %s", scenario.name, linkName)
			}
		}
	}
}

func (s *TestSearchUserSearch) differentValues() userSearchTestExpect {
	return func(t *testing.T, scenario okScenarioUserSearchTest, result *app.UserList) {
		var prev *app.UserData

		for i := range result.Data {
			u := result.Data[i]
			if prev == nil {
				prev = u
			} else {
				if *prev.Attributes.FullName == *u.Attributes.FullName {
					t.Errorf("%s got equal Fullname, wanted different %s", scenario.name, *u.Attributes.FullName)
				}
				if *prev.Attributes.ImageURL == *u.Attributes.ImageURL {
					t.Errorf("%s got equal ImageURL, wanted different %s", scenario.name, *u.Attributes.ImageURL)
				}
				if *prev.ID == *u.ID {
					t.Errorf("%s got equal ID, wanted different %s", scenario.name, *u.ID)
				}
				if prev.Type != u.Type {
					t.Errorf("%s got non equal Type, wanted same %s", scenario.name, u.Type)
				}
			}
		}
	}
}

func (s *TestSearchUserSearch) limit(n int) *int {
	return &n
}

func (s *TestSearchUserSearch) offset(n int) *string {
	str := strconv.Itoa(n)
	return &str
}
