setupCollections(db.getSiblingDB("MainDB"))
setupCollections(db.getSiblingDB("TestDB"))

function setupCollections(database) {
    database.createCollection("users");
    database.createCollection("chargepoints");
    database.createCollection("reservations");
}
