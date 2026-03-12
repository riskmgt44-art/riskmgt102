// handlers/delete_request_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"riskmgt/models"
	"riskmgt/utils"
)

// CreateDeleteRequest - Analyst submits a request to delete an action
func CreateDeleteRequest(w http.ResponseWriter, r *http.Request) {
	// Check collection
	if deleteRequestCollection == nil {
		log.Printf("[ERROR] CreateDeleteRequest: deleteRequestCollection is nil")
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete request service not initialized")
		return
	}

	// Get organization ID from context
	orgIDHex, ok := r.Context().Value("orgID").(string)
	if !ok || orgIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "Organization ID not found")
		return
	}

	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	// Get user info from context
	userIDHex, ok := r.Context().Value("userID").(string)
	if !ok || userIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "User ID not found")
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userName, _ := r.Context().Value("userName").(string)
	userRole, _ := r.Context().Value("userRole").(string)

	// Check if user is analyst (admins should use direct delete)
	if userRole == "admin" || userRole == "superadmin" {
		utils.RespondWithError(w, http.StatusForbidden, "Admins can delete actions directly")
		return
	}

	// Parse request body
	var req struct {
		ActionID    string `json:"actionId"`
		ActionTitle string `json:"actionTitle"`
		Reason      string `json:"reason"`
		Comments    string `json:"comments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	log.Printf("[DEBUG] CreateDeleteRequest: Received request: %+v", req)

	// Validate
	if req.ActionID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Action ID is required")
		return
	}
	if req.Reason == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Reason is required")
		return
	}

	// Parse action ID
	actionID, err := primitive.ObjectIDFromHex(req.ActionID)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid action ID format")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Check if action exists and is not already deleted
	var action models.Action
	err = actionCollection.FindOne(ctx, bson.M{
		"_id":            actionID,
		"organizationId": orgID,
		"isDeleted":      bson.M{"$ne": true},
	}).Decode(&action)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.RespondWithError(w, http.StatusNotFound, "Action not found or already deleted")
		} else {
			log.Printf("[ERROR] CreateDeleteRequest: Failed to find action: %v", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to verify action")
		}
		return
	}

	// Check if there's already a pending request for this action
	count, err := deleteRequestCollection.CountDocuments(ctx, bson.M{
		"organizationId": orgID,
		"actionId":       actionID,
		"status":         models.DeleteRequestStatusPending,
	})
	if err != nil {
		log.Printf("[ERROR] CreateDeleteRequest: Error checking existing requests: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to check existing requests")
		return
	}
	if count > 0 {
		utils.RespondWithError(w, http.StatusConflict, "A pending delete request already exists for this action")
		return
	}

	// Create delete request
	now := time.Now().UTC()
	deleteRequest := models.DeleteRequest{
		ID:              primitive.NewObjectID(),
		OrganizationID:  orgID,
		ActionID:        actionID,
		ActionTitle:     action.Title,
		RequestedBy:     userID,
		RequestedByName: userName,
		RequestDate:     now,
		Reason:          req.Reason,
		Comments:        req.Comments,
		Status:          models.DeleteRequestStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err = deleteRequestCollection.InsertOne(ctx, deleteRequest)
	if err != nil {
		log.Printf("[ERROR] CreateDeleteRequest: Failed to insert: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create delete request")
		return
	}

	// Create audit log
	auditDetails := bson.M{
		"actionId":    actionID.Hex(),
		"actionTitle": action.Title,
		"reason":      req.Reason,
		"requestId":   deleteRequest.ID.Hex(),
	}

	go CreateAuditLog(r.Context(), r, "DELETE_REQUEST_CREATE", "delete_request", deleteRequest.ID, auditDetails)

	log.Printf("[SUCCESS] CreateDeleteRequest: Created request %s for action %s", 
		deleteRequest.ID.Hex(), actionID.Hex())

	utils.RespondWithJSON(w, http.StatusCreated, deleteRequest)
}

// ListDeleteRequests - Get all delete requests (filtered by user role)
func ListDeleteRequests(w http.ResponseWriter, r *http.Request) {
	// Check collection
	if deleteRequestCollection == nil {
		log.Printf("[ERROR] ListDeleteRequests: deleteRequestCollection is nil")
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete request service not initialized")
		return
	}

	orgIDHex, ok := r.Context().Value("orgID").(string)
	if !ok || orgIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "Organization ID not found")
		return
	}

	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	userRole, _ := r.Context().Value("userRole").(string)
	userIDHex, _ := r.Context().Value("userID").(string)
	userID, _ := primitive.ObjectIDFromHex(userIDHex)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Build filter
	filter := bson.M{"organizationId": orgID}

	// If not admin, only show user's own requests
	if userRole != "admin" && userRole != "superadmin" {
		filter["requestedBy"] = userID
	}

	// Apply status filter if provided
	if status := r.URL.Query().Get("status"); status != "" && status != "all" {
		filter["status"] = status
	}

	// Pagination
	limit := int64(50)
	skip := int64(0)

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 64); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if skipStr := r.URL.Query().Get("skip"); skipStr != "" {
		if s, err := strconv.ParseInt(skipStr, 10, 64); err == nil && s >= 0 {
			skip = s
		}
	}

	opts := options.Find().
		SetSort(bson.D{{"requestDate", -1}}).
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := deleteRequestCollection.Find(ctx, filter, opts)
	if err != nil {
		log.Printf("[ERROR] ListDeleteRequests: Find failed: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch delete requests")
		return
	}
	defer cursor.Close(ctx)

	var requests []models.DeleteRequest
	if err = cursor.All(ctx, &requests); err != nil {
		log.Printf("[ERROR] ListDeleteRequests: Decode failed: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to decode delete requests")
		return
	}

	// Get total count
	total, _ := deleteRequestCollection.CountDocuments(ctx, filter)

	log.Printf("[SUCCESS] ListDeleteRequests: Returning %d requests", len(requests))

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"skip":     skip,
	})
}

// ReviewDeleteRequest - Admin approves or rejects a delete request
func ReviewDeleteRequest(w http.ResponseWriter, r *http.Request) {
	// Check collection
	if deleteRequestCollection == nil {
		log.Printf("[ERROR] ReviewDeleteRequest: deleteRequestCollection is nil")
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete request service not initialized")
		return
	}

	// Check if user is admin
	userRole, ok := r.Context().Value("userRole").(string)
	if !ok || (userRole != "admin" && userRole != "superadmin") {
		utils.RespondWithError(w, http.StatusForbidden, "Only admins can review delete requests")
		return
	}

	orgIDHex, ok := r.Context().Value("orgID").(string)
	if !ok || orgIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "Organization ID not found")
		return
	}

	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	// Get request ID from URL
	vars := mux.Vars(r)
	requestIDStr := vars["id"]
	if requestIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Request ID required")
		return
	}

	requestID, err := primitive.ObjectIDFromHex(requestIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request ID format")
		return
	}

	// Parse review data
	var review struct {
		Decision        string `json:"decision"` // "approved" or "rejected"
		RejectionReason string `json:"rejectionReason,omitempty"`
		Comments        string `json:"comments,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if review.Decision != "approved" && review.Decision != "rejected" {
		utils.RespondWithError(w, http.StatusBadRequest, "Decision must be 'approved' or 'rejected'")
		return
	}

	if review.Decision == "rejected" && review.RejectionReason == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Rejection reason is required when rejecting")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get the delete request
	var deleteRequest models.DeleteRequest
	err = deleteRequestCollection.FindOne(ctx, bson.M{
		"_id":            requestID,
		"organizationId": orgID,
		"status":         models.DeleteRequestStatusPending,
	}).Decode(&deleteRequest)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.RespondWithError(w, http.StatusNotFound, "Delete request not found or already processed")
		} else {
			log.Printf("[ERROR] ReviewDeleteRequest: Find failed: %v", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch delete request")
		}
		return
	}

	// Get reviewer info
	reviewerIDHex, _ := r.Context().Value("userID").(string)
	reviewerID, _ := primitive.ObjectIDFromHex(reviewerIDHex)
	reviewerName, _ := r.Context().Value("userName").(string)

	now := time.Now().UTC()

	// Update delete request
	update := bson.M{
		"status":         review.Decision,
		"reviewedBy":     reviewerID,
		"reviewedByName": reviewerName,
		"reviewDate":     now,
		"reviewComments": review.Comments,
		"updatedAt":      now,
	}

	if review.Decision == "rejected" {
		update["rejectionReason"] = review.RejectionReason
	}

	_, err = deleteRequestCollection.UpdateOne(ctx,
		bson.M{"_id": requestID},
		bson.M{"$set": update},
	)

	if err != nil {
		log.Printf("[ERROR] ReviewDeleteRequest: Update failed: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update delete request")
		return
	}

	// If approved, soft delete the action
	if review.Decision == "approved" {
		// Update action to mark as deleted
		actionUpdate := bson.M{
			"isDeleted":   true,
			"deletedBy":   reviewerID.Hex(),
			"deletedDate": now,
			"updatedAt":   now,
		}
		
		result, err := actionCollection.UpdateOne(ctx,
			bson.M{
				"_id":            deleteRequest.ActionID,
				"organizationId": orgID,
			},
			bson.M{"$set": actionUpdate},
		)

		if err != nil {
			log.Printf("[ERROR] ReviewDeleteRequest: Failed to soft delete action: %v", err)
		} else if result.MatchedCount == 0 {
			log.Printf("[WARN] ReviewDeleteRequest: Action %s not found for soft delete", deleteRequest.ActionID.Hex())
		} else {
			log.Printf("[SUCCESS] ReviewDeleteRequest: Soft deleted action %s", deleteRequest.ActionID.Hex())
		}
	}

	// Create audit log
	auditDetails := bson.M{
		"requestId":       requestID.Hex(),
		"actionId":        deleteRequest.ActionID.Hex(),
		"actionTitle":     deleteRequest.ActionTitle,
		"requestedBy":     deleteRequest.RequestedByName,
		"decision":        review.Decision,
		"rejectionReason": review.RejectionReason,
		"comments":        review.Comments,
	}

	go CreateAuditLog(r.Context(), r, "DELETE_REQUEST_"+review.Decision, "delete_request", requestID, auditDetails)

	log.Printf("[SUCCESS] ReviewDeleteRequest: Request %s %s", requestID.Hex(), review.Decision)

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Delete request " + review.Decision,
	})
}

// GetDeleteRequestStats - Get statistics about delete requests
func GetDeleteRequestStats(w http.ResponseWriter, r *http.Request) {
	// Check collection
	if deleteRequestCollection == nil {
		log.Printf("[ERROR] GetDeleteRequestStats: deleteRequestCollection is nil")
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete request service not initialized")
		return
	}

	orgIDHex, ok := r.Context().Value("orgID").(string)
	if !ok || orgIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "Organization ID not found")
		return
	}

	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get counts by status
	pipeline := mongo.Pipeline{
		{{"$match", bson.D{{"organizationId", orgID}}}},
		{{"$group", bson.D{
			{"_id", "$status"},
			{"count", bson.D{{"$sum", 1}}},
		}}},
	}

	cursor, err := deleteRequestCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("[ERROR] GetDeleteRequestStats: Aggregate failed: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get statistics")
		return
	}
	defer cursor.Close(ctx)

	stats := map[string]int{
		"pending":  0,
		"approved": 0,
		"rejected": 0,
		"total":    0,
	}

	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cursor.Decode(&result); err == nil {
			stats[result.ID] = result.Count
			stats["total"] += result.Count
		}
	}

	// Get pending requests count for admin badge
	userRole, _ := r.Context().Value("userRole").(string)
	if userRole == "admin" || userRole == "superadmin" {
		stats["pendingForApproval"] = stats["pending"]
	} else {
		// For analysts, get their pending requests count
		userIDHex, _ := r.Context().Value("userID").(string)
		userID, _ := primitive.ObjectIDFromHex(userIDHex)
		
		pendingCount, _ := deleteRequestCollection.CountDocuments(ctx, bson.M{
			"organizationId": orgID,
			"requestedBy":    userID,
			"status":         "pending",
		})
		stats["myPending"] = int(pendingCount)
	}

	utils.RespondWithJSON(w, http.StatusOK, stats)
}

// GetDeleteRequestByID - Get a single delete request
func GetDeleteRequestByID(w http.ResponseWriter, r *http.Request) {
	// Check collection
	if deleteRequestCollection == nil {
		log.Printf("[ERROR] GetDeleteRequestByID: deleteRequestCollection is nil")
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete request service not initialized")
		return
	}

	orgIDHex, ok := r.Context().Value("orgID").(string)
	if !ok || orgIDHex == "" {
		utils.RespondWithError(w, http.StatusUnauthorized, "Organization ID not found")
		return
	}

	orgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	vars := mux.Vars(r)
	requestIDStr := vars["id"]
	if requestIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Request ID required")
		return
	}

	requestID, err := primitive.ObjectIDFromHex(requestIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request ID format")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var deleteRequest models.DeleteRequest
	err = deleteRequestCollection.FindOne(ctx, bson.M{
		"_id":            requestID,
		"organizationId": orgID,
	}).Decode(&deleteRequest)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.RespondWithError(w, http.StatusNotFound, "Delete request not found")
		} else {
			log.Printf("[ERROR] GetDeleteRequestByID: Find failed: %v", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch delete request")
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, deleteRequest)
}