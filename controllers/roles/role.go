package roles

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"

  f "github.com/go-playground/form"

  bulky "github.com/charmixer/bulky/client"

  "github.com/gorilla/csrf"
)

type formInput struct {
  Name          string
  Description   string
}

func ShowRole(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowRole",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    c.HTML(http.StatusOK, "role.html", gin.H{
      "title": "Create new role",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitRole(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitRole",
    })

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

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    var createRolesRequests []idp.CreateRolesRequest

    createRolesRequests = append(createRolesRequests, idp.CreateRolesRequest{
      Name: form.Name,
      Description: form.Description,
    })

    url := config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.roles.collection")
    httpStatus, responses, err := idp.CreateRoles(idpClient, url, createRolesRequests)

    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }

    if httpStatus != 200 {
      log.Debug("Failed to get 200 from " + url);
      c.AbortWithStatus(404)
      return
    }

    var createRolesResponse idp.CreateRolesResponse
    restStatus, restErr := bulky.Unmarshal(0, responses, &createRolesResponse)

    if restErr != nil {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(404)
      return
    }

    if restStatus == 200 {
      c.Redirect(http.StatusFound, "/roles")
      c.Abort()
      return
    }

    c.Redirect(http.StatusFound, "/role")
  }
  return gin.HandlerFunc(fn)
}
