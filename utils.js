const cassandra = require('cassandra-driver');

// Create Cassandra client
const client = new cassandra.Client({
    contactPoints: ['127.0.0.1'], // Replace with your ScyllaDB IP if necessary
    localDataCenter: 'datacenter1',
    keyspace: 'shortenurl'
});

// Utility function to generate random ID
function makeID(length) {
    let result = '';
    const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    const charactersLength = characters.length;
    for (let i = 0; i < length; i++) {
        result += characters.charAt(Math.floor(Math.random() * charactersLength));
    }
    return result;
}

// Function to find the original URL
async function findOrigin(id, memJsClient) {
    try {
        const cachedResult = await memJsClient.get(id);
        if (cachedResult.value) {
            return cachedResult.value.toString();
        }

        const query = 'SELECT url FROM urls WHERE id = ?';
        const result = await client.execute(query, [id], { prepare: true });

        if (result.rows.length > 0) {
            const url = result.rows[0].url;
            await memJsClient.set(id, url);
            return url;
        } else {
            return null;
        }
    } catch (error) {
        throw new Error(error);
    }
}

// Function to create a short URL
async function create(id, url) {
    try {
        const query = 'INSERT INTO urls (id, url) VALUES (?, ?)';
        await client.execute(query, [id, url], { prepare: true });
        return id;
    } catch (error) {
        throw new Error(error);
    }
}

// Function to shorten the URL
async function shortUrl(url, memJsClient) {
    const newID = makeID(5);
    const originUrl = await findOrigin(newID, memJsClient);
    if (!originUrl) {
        await create(newID, url);
    }
    return newID;
}

module.exports = {
    findOrigin,
    shortUrl
};
