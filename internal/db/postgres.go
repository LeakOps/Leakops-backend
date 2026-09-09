package db

import(
	"log"

	"Leakops-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)


func connectDB(dsn *string) *gorm.DB {
	database, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{})

	if err != nil {
		log.Fatal("Failed to connect with database: ", err)
	}

	log.Println("Database connected successfully")

	// Auto-migrate all models - create tables if they don't exist
	err = database.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.GatewayAccount{},
		&models.FailedPayment{},
		&models.RetryLog{},
	)

	if err != nil {
		log.Fatal("Auto migration failed: ", err)
	}

	log.Println("Migration completed successfully!!")

	return database
}
