package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func registerReconcileRoutes(apiRouter *gin.RouterGroup) {
	route := apiRouter.Group("/reconcile")
	route.Use(middleware.AdminAuth())

	route.GET("/configs", middleware.RequirePermission(authz.ReconcileRead), controller.ListReconcileConfigs)
	route.GET("/configs/:id", middleware.RequirePermission(authz.ReconcileRead), controller.GetReconcileConfig)
	route.POST("/configs", middleware.RequirePermission(authz.ReconcileConfigure), controller.CreateReconcileConfig)
	route.PUT("/configs/:id", middleware.RequirePermission(authz.ReconcileConfigure), controller.UpdateReconcileConfig)
	route.DELETE("/configs/:id", middleware.RequirePermission(authz.ReconcileConfigure), controller.DeleteReconcileConfig)
}
