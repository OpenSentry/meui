package grant

import (
  "net/http"
  "time"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  "golang.org/x/oauth2"
  oidc "github.com/coreos/go-oidc/v3/oidc"

  bulky "github.com/charmixer/bulky/client"

  aap "github.com/opensentry/aap/client"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"

  "github.com/opensentry/meui/app"
  f "github.com/go-playground/form"
  "fmt"
)

type formInput struct {
  Publisher     string
  Receiver       string
  Grants        []struct{
    Scope          string
    Enabled        bool
    StartDate      string
    EndDate        string
  }
}

type uiGrant struct {
  Nbf string
  Exp string
  Granted bool
}

func ShowGrants(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowGrants",
    })

    session := sessions.Default(c)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    publisher, publisherExists := c.GetQuery("publisher")
    receiver, receiverExists := c.GetQuery("receiver")

    if !receiverExists {
      receiver = identity.Id
    }

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "grants.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)
    idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    var url string
    var responses []bulky.Response
    var err error
    var restErr []bulky.ErrorResponse

    // fetch publishes
    var publishes aap.ReadPublishesResponse
    var grantPublishes []aap.Publish
    var mayGrantPublishes []aap.Publish
    if publisherExists {
      url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.publishes")
      _, responses, err = aap.ReadPublishes(aapClient, url, []aap.ReadPublishesRequest{
        {Publisher: publisher},
      })

      if err != nil {
        c.AbortWithStatus(404)
        log.Debug(err.Error())
        return
      }

      _, restErr = bulky.Unmarshal(0, responses, &publishes)
      if len(restErr) > 0 {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }

        c.AbortWithStatus(404)
        return
      }

      for _,p := range publishes {
        if len(p.MayGrantScopes) > 0 {
          mayGrantPublishes = append(mayGrantPublishes, p)
          continue
        }

        grantPublishes = append(grantPublishes, p)
      }
    }

    // fetch grants

    url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.grants")
    _, responses, err = aap.ReadGrants(aapClient, url, []aap.ReadGrantsRequest{
      { Identity: receiver, Publisher: publisher},
    })

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    var grants aap.ReadGrantsResponse
    _, restErr = bulky.Unmarshal(0, responses, &grants)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    var hasGrantsMap = make(map[string]uiGrant, len(grants))
    for _,g := range grants {

      var nbf string
      if g.NotBefore != 0 {
        nbf = time.Unix(g.NotBefore, 0).Format("2006-01-02")
      }

      var exp string
      if g.Expire != 0 {
        exp = time.Unix(g.Expire, 0).Format("2006-01-02")
      }

      hasGrantsMap[g.Scope] = uiGrant{
        Nbf: nbf,
        Exp: exp,
        Granted: true,
      }
    }

    // fetch resourceservers

    url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection")
    _, responses, err = idp.ReadResourceServers(idpClient, url, nil)

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    var resourceservers idp.ReadResourceServersResponse
    _, restErr = bulky.Unmarshal(0, responses, &resourceservers)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    c.HTML(200, "grants.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "title": "Grants",
      "hasGrantsMap": hasGrantsMap,
      "grantPublishes": grantPublishes,
      "mayGrantPublishes": mayGrantPublishes,
      "resourceservers": resourceservers,
      "publisher": publisher,
      "receiver": receiver,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitGrants(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitGrants",
    })

    session := sessions.Default(c)

    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "grants.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    publisher, publisherExists := c.GetQuery("publisher")
    receiver, receiverExists := c.GetQuery("receiver")

    if !publisherExists || !receiverExists  {
      log.WithFields(logrus.Fields{
        "publisher": publisher,
        "receiver": receiver,
      }).Debug("publisher and receiver must exists")
      c.AbortWithStatus(404)
      return
    }

    var form formInput
    c.Request.ParseForm()

    decoder := f.NewDecoder()

    // must pass a pointer
    err := decoder.Decode(&form, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(404)
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)

    var createGrantsRequests []aap.CreateGrantsRequest
    var deleteGrantsRequests []aap.DeleteGrantsRequest
    for _,grant := range form.Grants {
      layout := "2006-01-02"

      var nbf int64
      if grant.StartDate != "" {
        nbfTime, err := time.Parse(layout, grant.StartDate)
        if err != nil {
           panic(err)
        }

        nbf = nbfTime.Unix()
      }

      var exp int64
      if grant.EndDate != "" {
        expTime, err := time.Parse(layout, grant.EndDate)
        if err != nil {
           panic(err)
        }

        exp = expTime.Unix()
      }

      if grant.Enabled {
        createGrantsRequests = append(createGrantsRequests, aap.CreateGrantsRequest{
          Identity: receiver,
          Scope: grant.Scope,
          Publisher: publisher,
          OnBehalfOf: publisher, // TODO FIXME this should be something you can choose from the gui (data scoped access rights)
          NotBefore: nbf,
          Expire: exp,
        })
        continue;
      }

      // deny by default
      deleteGrantsRequests = append(deleteGrantsRequests, aap.DeleteGrantsRequest{
        Identity: receiver,
        Scope: grant.Scope,
        Publisher: publisher,
        OnBehalfOf: publisher, // TODO FIXME this should be something you can choose from the gui (data scoped access rights)
      })
    }

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.grants")

    var createStatus int
    var createResponses []bulky.Response
    if createGrantsRequests != nil {
      createStatus, createResponses, err = aap.CreateGrants(aapClient, url, createGrantsRequests)
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(404)
        return
      }
    }

    var deleteStatus int
    var deleteResponses []bulky.Response
    if deleteGrantsRequests != nil {
      deleteStatus, deleteResponses, err = aap.DeleteGrants(aapClient, url, deleteGrantsRequests)
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(404)
        return
      }
    }

    foundErrors := false

    if createStatus == 200 {
      var createGrants aap.CreateGrantsResponse
      _, restErr := bulky.Unmarshal(0, createResponses, &createGrants)
      if restErr != nil {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }
        foundErrors = true
      }
    }

    if deleteStatus == 200 {
      var deleteGrants aap.DeleteGrantsResponse
      _, restErr := bulky.Unmarshal(0, deleteResponses, &deleteGrants)
      if restErr != nil {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }
        foundErrors = true
      }

    }

    if !foundErrors {
      c.Redirect(http.StatusFound, fmt.Sprintf("/access/grant?receiver=%s&publisher=%s", receiver, publisher))
      c.Abort()
      return
    }

    c.AbortWithStatus(404)
  }
  return gin.HandlerFunc(fn)
}
