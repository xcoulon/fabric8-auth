package controller_test

import (
	"testing"

	"context"
	"github.com/fabric8-services/fabric8-auth/app"
	"github.com/fabric8-services/fabric8-auth/app/test"
	"github.com/fabric8-services/fabric8-auth/controller"
	"github.com/fabric8-services/fabric8-auth/gormtestsupport"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/fabric8-services/fabric8-auth/resource"
	testsupport "github.com/fabric8-services/fabric8-auth/test"

	"github.com/goadesign/goa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LogoutControllerTestSuite struct {
	gormtestsupport.DBTestSuite
}

func TestLogoutControllerTestSuite(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &LogoutControllerTestSuite{DBTestSuite: gormtestsupport.NewDBTestSuite()})
}

func (s *LogoutControllerTestSuite) UnSecuredController() (*goa.Service, *controller.LogoutController) {
	svc := testsupport.ServiceAsUser("Logout-Service", testsupport.TestIdentity)
	return svc, controller.NewLogoutController(svc, s.Application)
}

func (s *LogoutControllerTestSuite) TestLogoutRedirects() {
	// given
	svc, ctrl := s.UnSecuredController()
	redirect := "http://domain.com"
	// when
	resp := test.LogoutLogoutTemporaryRedirect(s.T(), svc.Context, svc, ctrl, &redirect)
	// then
	assert.Equal(s.T(), resp.Header().Get("Cache-Control"), "no-cache")
}

func (s *LogoutControllerTestSuite) TestLogoutWithoutReffererAndRedirectParamsBadRequest() {
	// given
	svc, ctrl := s.UnSecuredController()
	// when/then
	test.LogoutLogoutBadRequest(s.T(), svc.Context, svc, ctrl, nil)
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithRedirectParam() {
	s.checkRedirects("https://openshift.io/home", "", "https%3A%2F%2Fopenshift.io%2Fhome")
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithReferrer() {
	s.checkRedirects("", "https://openshift.io/home", "https%3A%2F%2Fopenshift.io%2Fhome")
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithReferrerAndRedirect() {
	s.checkRedirects("https://prod-preview.openshift.io/home", "https://url.example.org/path", "https%3A%2F%2Fprod-preview.openshift.io%2Fhome")
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithInvalidRedirectParamBadRequest() {
	s.checkRedirects("https://url.example.org/path", "", "")
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithInvalidReferrerParamBadRequest() {
	s.checkRedirects("", "https://url.example.org/path", "")
}

func (s *LogoutControllerTestSuite) TestLogoutRedirectsWithReferrerAndInvalidRedirectBadRequest() {
	s.checkRedirects("https://url.example.org/path", "https://openshift.io/home", "")
}

func (s *LogoutControllerTestSuite) checkRedirects(redirectParam string, referrerURL string, expectedRedirectParam string) {
	rw := httptest.NewRecorder()
	u := &url.URL{
		Path: "/api/logout",
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	require.Nil(s.T(), err)
	if referrerURL != "" {
		req.Header.Add("referer", referrerURL)
	}

	prms := url.Values{}
	if redirectParam != "" {
		prms.Add("redirect", redirectParam)
	}
	ctx := context.Background()
	goaCtx := goa.NewContext(goa.WithAction(ctx, "LogoutTest"), rw, req, prms)
	logoutCtx, err := app.NewLogoutLogoutContext(goaCtx, req, goa.New("LogoutService"))
	require.Nil(s.T(), err)

	svc, ctrl := s.UnSecuredController()

	test.LogoutLogoutTemporaryRedirect(s.T(), logoutCtx, svc, ctrl, &expectedRedirectParam)

	if expectedRedirectParam == "" {
		assert.Equal(s.T(), 400, rw.Code)
	} else {
		assert.Equal(s.T(), 307, rw.Code)
		assert.Equal(s.T(), expectedRedirectParam, rw.Header().Get("Location"))
	}
}