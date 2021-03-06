package profiles

import (
  "net/http"
  "net/url"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
)

func ShowProfile(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowProfile",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    publicProfileUrl, err := url.Parse(config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.profile"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    q := publicProfileUrl.Query()
    q.Add("id", identity.Id)
    publicProfileUrl.RawQuery = q.Encode()

    c.HTML(http.StatusOK, "profile.html", gin.H{
      "title": "Profile",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "password": identity.Password,
      "name": identity.Name,
      "email": identity.Email,
      "totp_required": identity.TotpRequired,
      "meUiUrl": config.GetString("meui.public.url"),
      "changeEmailUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.emailchange"),
      "changePasswordUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.password"),
      "profileDeleteUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.delete"),
      "setupTotpUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.totp"),
      "logoutUrl": config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.logout"),
      "publicProfileUrl": publicProfileUrl.String(),
    })
    return
  }
  return gin.HandlerFunc(fn)
}
