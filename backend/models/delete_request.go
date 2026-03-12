// models/delete_request.go
package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/bson" // Add this import
)

// DeleteRequestStatus represents the status of a delete request
type DeleteRequestStatus string

const (
	DeleteRequestStatusPending  DeleteRequestStatus = "pending"
	DeleteRequestStatusApproved DeleteRequestStatus = "approved"
	DeleteRequestStatusRejected DeleteRequestStatus = "rejected"
)

// DeleteRequest represents a request to delete an action
type DeleteRequest struct {
	ID              primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	OrganizationID  primitive.ObjectID   `json:"organizationId" bson:"organizationId"`
	ActionID        primitive.ObjectID   `json:"actionId" bson:"actionId"`
	ActionTitle     string                `json:"actionTitle" bson:"actionTitle"`
	RequestedBy     primitive.ObjectID   `json:"requestedBy" bson:"requestedBy"`
	RequestedByName string                `json:"requestedByName" bson:"requestedByName"`
	RequestDate     time.Time             `json:"requestDate" bson:"requestDate"`
	Reason          string                `json:"reason" bson:"reason"`
	Comments        string                `json:"comments,omitempty" bson:"comments,omitempty"`
	Status          DeleteRequestStatus   `json:"status" bson:"status"`
	
	// Review fields
	ReviewedBy      *primitive.ObjectID  `json:"reviewedBy,omitempty" bson:"reviewedBy,omitempty"`
	ReviewedByName  string                `json:"reviewedByName,omitempty" bson:"reviewedByName,omitempty"`
	ReviewDate      *time.Time            `json:"reviewDate,omitempty" bson:"reviewDate,omitempty"`
	RejectionReason string                `json:"rejectionReason,omitempty" bson:"rejectionReason,omitempty"`
	ReviewComments  string                `json:"reviewComments,omitempty" bson:"reviewComments,omitempty"`
	
	// Timestamps
	CreatedAt       time.Time             `json:"createdAt" bson:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt" bson:"updatedAt"`
}

// ToBSON converts the DeleteRequest to BSON for database storage
func (d *DeleteRequest) ToBSON() bson.M {
	return bson.M{
		"_id":              d.ID,
		"organizationId":   d.OrganizationID,
		"actionId":         d.ActionID,
		"actionTitle":      d.ActionTitle,
		"requestedBy":      d.RequestedBy,
		"requestedByName":  d.RequestedByName,
		"requestDate":      d.RequestDate,
		"reason":           d.Reason,
		"comments":         d.Comments,
		"status":           d.Status,
		"reviewedBy":       d.ReviewedBy,
		"reviewedByName":   d.ReviewedByName,
		"reviewDate":       d.ReviewDate,
		"rejectionReason":  d.RejectionReason,
		"reviewComments":   d.ReviewComments,
		"createdAt":        d.CreatedAt,
		"updatedAt":        d.UpdatedAt,
	}
}