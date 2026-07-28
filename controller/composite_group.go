package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type compositeGroupStatusRequest struct {
	Status int `json:"status"`
}

func AdminListCompositeGroups(c *gin.Context) {
	groups, err := model.GetAllCompositeGroups(model.DB)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func AdminGetCompositeGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.GetCompositeGroupByID(model.DB, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func AdminCreateCompositeGroup(c *gin.Context) {
	var group model.CompositeGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.ValidateCompositeGroup(group, group.Routes); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := model.CreateCompositeGroup(model.DB, &group, group.Routes); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RefreshCompositeGroupCache(); err != nil {
		common.ApiError(c, err)
		return
	}
	created, err := model.GetCompositeGroupByID(model.DB, group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, created)
}

func AdminUpdateCompositeGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var group model.CompositeGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		common.ApiError(c, err)
		return
	}
	group.Id = id
	if err := service.ValidateCompositeGroup(group, group.Routes); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := model.UpdateCompositeGroup(model.DB, &group, group.Routes); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RefreshCompositeGroupCache(); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.GetCompositeGroupByID(model.DB, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func AdminUpdateCompositeGroupStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request compositeGroupStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Status != 0 && request.Status != 1 {
		common.ApiErrorMsg(c, "status must be 0 or 1")
		return
	}
	if request.Status == 1 {
		group, err := model.GetCompositeGroupByID(model.DB, id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if err := service.ValidateCompositeGroup(*group, group.Routes); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	}
	if err := model.UpdateCompositeGroupStatus(model.DB, id, request.Status); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RefreshCompositeGroupCache(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminValidateCompositeGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.GetCompositeGroupByID(model.DB, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.ValidateCompositeGroup(*group, group.Routes); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminDeleteCompositeGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteCompositeGroup(model.DB, id); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RefreshCompositeGroupCache(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
