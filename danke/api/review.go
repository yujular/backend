package api

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	. "github.com/opentreehole/backend/common"
	"github.com/opentreehole/backend/common/sensitive"
	. "github.com/opentreehole/backend/danke/model"
	. "github.com/opentreehole/backend/danke/schema"
	"gorm.io/gorm"
	"time"
)

// GetReviewV1 godoc
// @Summary get a review
// @Description get a review
// @Tags Review
// @Accept json
// @Produce json
// @Router /reviews/{id} [get]
// @Param id path int true "review id"
// @Success 200 {object} schema.ReviewV1Response
// @Failure 400 {object} common.HttpError
func GetReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	review, err := FindReviewByID(DB, id, FindReviewOption{
		PreloadHistory:     true,
		PreloadAchievement: true,
		PreloadVote:        true,
		UserID:             user.ID,
	})
	if err != nil {
		return
	}

	return c.JSON(new(ReviewV1Response).FromModel(user, review))
}

// ListReviewsV1 godoc
// @Summary list reviews
// @Description list reviews
// @Tags Review
// @Accept json
// @Produce json
// @Router /courses/{id}/reviews [get]
// @Param id path int true "course id"
// @Success 200 {array} schema.ReviewV1Response
// @Failure 400 {object} common.HttpError
func ListReviewsV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return
	}

	courseID, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	// 查找评论
	var reviews ReviewList
	err = DB.Find(&reviews, "course_id = ?", courseID).Error
	if err != nil {
		return
	}

	// 加载评论投票
	err = reviews.LoadVoteListByUserID(user.ID)
	if err != nil {
		return
	}

	// 创建 response
	response := make([]*ReviewV1Response, 0, len(reviews))
	for _, review := range reviews {
		response = append(response, new(ReviewV1Response).FromModel(user, review))
	}

	return c.JSON(response)
}

// CreateReviewV1 godoc
// @Summary create a review
// @Description create a review
// @Tags Review
// @Accept json
// @Produce json
// @Param json body schema.CreateReviewV1Request true "json"
// @Param course_id path int true "course id"
// @Router /courses/{course_id}/reviews [post]
// @Success 200 {object} schema.ReviewV1Response
// @Failure 400 {object} common.HttpError
// @Failure 404 {object} common.HttpBaseError
func CreateReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return
	}

	var req CreateReviewV1Request
	err = ValidateBody(c, &req)
	if err != nil {
		return
	}

	courseID, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	// 创建评论
	review := req.ToModel(user.ID, courseID)

	sensitiveResp, err := sensitive.CheckSensitive(sensitive.ParamsForCheck{
		Content:  req.Title + "\n" + req.Content,
		Id:       time.Now().UnixNano(),
		TypeName: sensitive.TypeTitle,
	})
	if err != nil {
		return
	}
	review.IsSensitive = !sensitiveResp.Pass
	review.IsActuallySensitive = nil
	review.SensitiveDetail = sensitiveResp.Detail

	err = review.Create(DB)
	if err != nil {
		return
	}

	return c.JSON(new(ReviewV1Response).FromModel(user, review))
}

// ModifyReviewV1 godoc
// @Summary modify a review
// @Description modify a review, admin or owner can modify
// @Tags Review
// @Accept json
// @Produce json
// @Param json body schema.ModifyReviewV1Request true "json"
// @Param review_id path int true "review id"
// @Router /reviews/{review_id} [put]
// @Success 200 {object} schema.ReviewV1Response
// @Failure 400 {object} common.HttpError
// @Failure 404 {object} common.HttpBaseError
func ModifyReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return err
	}

	var req ModifyReviewV1Request
	err = ValidateBody(c, &req)
	if err != nil {
		return
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	// 查找评论
	review, err := FindReviewByID(DB, id)
	if err != nil {
		return
	}

	// 检查权限
	if !user.IsAdmin && review.ReviewerID != user.ID {
		return Forbidden("没有权限")
	}

	sensitiveResp, err := sensitive.CheckSensitive(sensitive.ParamsForCheck{
		Content:  req.Title + "\n" + req.Content,
		Id:       time.Now().UnixNano(),
		TypeName: sensitive.TypeTitle,
	})
	if err != nil {
		return err
	}

	var newReview = Review{
		IsSensitive:         !sensitiveResp.Pass,
		IsActuallySensitive: nil,
		SensitiveDetail:     sensitiveResp.Detail,
		Content:             req.Content,
		Title:               req.Title,
		Rank:                req.Rank.ToModel(),
	}

	// 修改评论
	err = review.Update(DB, newReview)
	if err != nil {
		return
	}

	// 查找评论
	review, err = FindReviewByID(DB, id, FindReviewOption{
		PreloadHistory:     true,
		PreloadAchievement: true,
		PreloadVote:        true,
		UserID:             user.ID,
	})
	if err != nil {
		return
	}

	return c.JSON(new(ReviewV1Response).FromModel(user, review))
}

// DeleteReviewV1 godoc
// @Summary delete a review
// @Description delete a review, admin or owner can delete
// @Tags Review
// @Accept json
// @Produce json
// @Param review_id path int true "review id"
// @Router /reviews/{review_id} [delete]
// @Success 204
// @Failure 400 {object} common.HttpError
// @Failure 403 {object} common.HttpError
// @Failure 404 {object} common.HttpError
func DeleteReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return err
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	// 查找评论
	review, err := FindReviewByID(DB, id)
	if err != nil {
		return
	}

	// 检查权限
	if !user.IsAdmin && review.ReviewerID != user.ID {
		return Forbidden("没有权限")
	}

	// 删除评论
	err = DB.Delete(review).Error
	if err != nil {
		return
	}

	return c.Status(204).JSON(nil)
}

// VoteForReviewV1 godoc
// @Summary vote for a review
// @Description vote for a review
// @Tags Review
// @Accept json
// @Produce json
// @Param json body schema.VoteForReviewV1Request true "json"
// @Param review_id path int true "review id"
// @Router /reviews/{review_id} [patch]
// @Success 200 {object} schema.ReviewV1Response
// @Failure 400 {object} common.HttpError
// @Failure 404 {object} common.HttpBaseError
func VoteForReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return err
	}

	var req VoteForReviewV1Request
	err = ValidateBody(c, &req)
	if err != nil {
		return
	}

	reviewID, err := c.ParamsInt("id")
	if err != nil {
		return
	}

	// 查找评论
	review, err := FindReviewByID(DB, reviewID)
	if err != nil {
		return err
	}

	err = DB.Transaction(func(tx *gorm.DB) (err error) {
		// 获取用户投票
		var vote ReviewVote
		err = tx.Where("review_id = ? AND user_id = ?", reviewID, user.ID).First(&vote).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
		}

		if req.Upvote {
			if vote.Data == 1 {
				vote.Data = 0
			} else {
				vote.Data = 1
			}
		} else {
			if vote.Data == -1 {
				vote.Data = 0
			} else {
				vote.Data = -1
			}
		}

		vote.UserID = user.ID
		vote.ReviewID = reviewID

		// 更新投票
		err = tx.Save(&vote).Error
		if err != nil {
			return
		}

		// 更新评论投票数
		err = tx.Model(&review).
			UpdateColumns(map[string]any{
				"upvote_count": tx.
					Model(&ReviewVote{}).
					Where("review_id = ? AND data = 1", reviewID).
					Select("count(*)"),
				"downvote_count": tx.
					Model(&ReviewVote{}).
					Where("review_id = ? AND data = -1", reviewID).
					Select("count(*)"),
			}).Error
		return
	})
	if err != nil {
		return
	}

	// 查找评论
	review, err = FindReviewByID(DB, reviewID, FindReviewOption{
		PreloadHistory:     true,
		PreloadAchievement: true,
		PreloadVote:        true,
		UserID:             user.ID,
	})
	if err != nil {
		return err
	}

	return c.JSON(new(ReviewV1Response).FromModel(user, review))
}

// ListMyReviewsV1 godoc
// @Summary list my reviews
// @Description list my reviews, old version. load history and achievements, no `is_me` field
// @Tags Review
// @Accept json
// @Produce json
// @Router /reviews/me [get]
// @Success 200 {array} schema.MyReviewV1Response
// @Failure 400 {object} common.HttpError
// @Failure 404 {object} common.HttpBaseError
func ListMyReviewsV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return err
	}

	// 查找评论
	var reviews ReviewList
	err = DB.Find(&reviews, "reviewer_id = ?", user.ID).Error
	if err != nil {
		return
	}

	// 加载评论投票
	err = reviews.LoadVoteListByUserID(user.ID)
	if err != nil {
		return
	}

	// 创建 response
	response := make([]*MyReviewV1Response, 0, len(reviews))
	for _, review := range reviews {
		response = append(response, new(MyReviewV1Response).FromModel(review))
	}

	return c.JSON(response)
}

// GetRandomReviewV1 godoc
// @Summary get random review
// @Description get random review
// @Tags Review
// @Accept json
// @Produce json
// @Router /reviews/random [get]
// @Success 200 {object} schema.RandomReviewV1Response
// @Failure 400 {object} common.HttpError
// @Failure 404 {object} common.HttpBaseError
func GetRandomReviewV1(c *fiber.Ctx) (err error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return err
	}

	// 获取随机评论
	var review Review
	if DB.Dialector.Name() == "mysql" {
		err = DB.Preload("Course").Joins(`JOIN (SELECT ROUND(RAND() * ((SELECT MAX(id) FROM review) - (SELECT MIN(id) FROM review)) + (SELECT MIN(id) FROM review)) AS id) AS number_table`).
			Where("review.id >= number_table.id").Limit(1).First(&review).Error
	} else {
		err = DB.Order("RANDOM()").Limit(1).First(&review).Error
	}
	if err != nil {
		return
	}

	// 加载评论投票
	err = review.LoadVoteListByUserID(user.ID)
	if err != nil {
		return
	}

	return c.JSON(new(RandomReviewV1Response).FromModel(&review))
}
