const sqlite3 = require('sqlite3').verbose();
const db = new sqlite3.Database('./db/app.db');

db.run(`
        CREATE TABLE IF NOT EXISTS data(
        id TEXT,
        url TEXT
        ) STRICT
`);

function makeID(length) {
    let result = '';
    const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    const charactersLength = characters.length;
    let counter = 0;
    while (counter < length) {
        result += characters.charAt(Math.floor(Math.random() * charactersLength));
        counter += 1;
    }
    return result;
}

function findOrigin(id, memJsClient) {
    return new Promise(async (resolve, reject) => {
        let result = await memJsClient.get(id)
        if (result.value) {
            return resolve(result.value.toString())
        } else {
            db.get(`SELECT * FROM data WHERE id = ?`, [id], function (err, res) {
                if (err) {
                    return reject(err.message);
                }
                if (res !== undefined) {
                    memJsClient.set(id, res.url).then()
                    return resolve(res.url);
                } else {
                    return resolve(null);
                }
            });

        }
    })
}

function create(id, url) {
    return new Promise((resolve, reject) => {
        return db.run(`INSERT INTO data VALUES (?, ?)`, [id, url], function (err) {
            if (err) {
                return reject(err.message);
            }
            return resolve(id);
        });
    });
}

async function shortUrl(url) {
    let newID = makeID(5);
    let originUrl = await findOrigin(newID);
    if (originUrl == null) await create(newID, url)
    return newID;
}

let mem_js = require('memjs')
module.exports = {
    findOrigin,
    shortUrl,
    mem_js
}