package postgres

import (
	"database/sql"
	"sync"
	"time"
)

const (
	maxOpenConnections = 5
	maxIdleConnections = 1
	maxLifeTimeSecond  = 60 * time.Second
)

// Pool saved structure per postgres engine configuration.
type poolSaved struct {
	// Username and password are saved because this comes from secret
	username string
	password string
	// This map will save all pools per database
	pools *sync.Map
}

// Pool manager map per pgec.
var poolManagerStorage sync.Map = sync.Map{}

func getOrOpenPool(p *pg, database string) (*sql.DB, error) {
	// Create result
	var db *sql.DB

	// Check if there is a saved pool in the storage
	savInt, ok := poolManagerStorage.Load(p.GetName())
	// Check if this is found
	if !ok {
		// Open connection
		sqlDB, err := openConnection(p, database)
		// Check error
		if err != nil {
			return nil, err
		}

		// Create saved pool map with the selected database
		psMap := &sync.Map{}
		psMap.Store(database, sqlDB)
		// Add it to storage
		poolManagerStorage.Store(p.GetName(), &poolSaved{
			username: p.GetUser(),
			password: p.GetPassword(),
			pools:    psMap,
		})

		// Result
		db = sqlDB
	} else {
		// Cast saved pool object
		sav := savInt.(*poolSaved)
		// Check if username and password haven't changed, if yes, close pools and recreate current
		if sav.username != p.GetUser() || sav.password != p.GetPassword() {
			// Close all pools
			err := CloseAllSavedPoolsForName(p.GetName())
			// Check error
			if err != nil {
				return nil, err
			}
		}
		// Check if we can found a pool for this database
		sqlDBInt, ok := sav.pools.Load(database)
		// Check if it isn't found
		if !ok {
			// Open connection
			sqlDB, err := openConnection(p, database)
			// Check error
			if err != nil {
				return nil, err
			}

			// Save it in pool manager storage
			sav.pools.Store(database, sqlDB)

			// Result
			db = sqlDB
		} else {
			// Result
			db = sqlDBInt.(*sql.DB)
		}
	}

	return db, nil
}

func openConnection(p *pg, database string) (*sql.DB, error) {
	// Generate url
	pgURL := TemplatePostgresqlURLWithArgs(
		p.GetHost(),
		p.GetUser(),
		p.GetPassword(),
		p.GetArgs(),
		database,
		p.GetPort(),
	)
	// Connect
	db, err := sql.Open("postgres", pgURL)
	// Check error
	if err != nil {
		return nil, err
	}

	// Set sql parameters
	// Force connections to 60s max lifetime because operator shouldn't take a slot too longer
	db.SetConnMaxLifetime(maxLifeTimeSecond)
	// Operator shouldn't take too much slots
	db.SetMaxIdleConns(maxIdleConnections)
	// Operator shouldn't take too much slots
	db.SetMaxOpenConns(maxOpenConnections)

	return db, nil
}

func CloseDatabaseSavedPoolsForName(name, database string) error {
	// Get pool saved
	psInt, ok := poolManagerStorage.Load(name)
	// Check if it exists
	if !ok {
		return nil
	}

	// Cast pool saved
	ps := psInt.(*poolSaved)

	// Get entry
	enInt, ok := ps.pools.Load(database)
	// Check if it isn't present
	if !ok {
		return nil
	}

	// Cast db
	en := enInt.(*sql.DB)

	// Close pool
	err := en.Close()
	// Check error
	if err != nil {
		return err
	}

	// Clean entry
	ps.pools.Delete(database)

	return nil
}

func CloseAllSavedPoolsForName(name string) error {
	// Get pool saved
	psInt, ok := poolManagerStorage.Load(name)
	// Check if it exists
	if !ok {
		return nil
	}

	// Cast pool saved
	ps := psInt.(*poolSaved)

	// Save all keys to be removed
	keysToBeRemoved := make([]interface{}, 0)
	// Error
	var err error
	// Loop over pools
	ps.pools.Range(func(k, val interface{}) bool {
		// Cast sql db
		v := val.(*sql.DB)
		// Close pool
		err = v.Close()
		// Check error
		if err != nil {
			return false
		}

		// Save key to be removed
		keysToBeRemoved = append(keysToBeRemoved, k)

		// Default
		return true
	})
	// Loop over keys to remove
	for _, v := range keysToBeRemoved {
		// Delete key
		ps.pools.Delete(v)
	}
	// Check error
	if err != nil {
		return nil
	}

	// Clean main entry
	poolManagerStorage.Delete(name)

	return nil
}
