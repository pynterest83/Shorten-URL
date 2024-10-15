const express = require('hyper-express')
const lib = require('./utils')
const mem_js = require("memjs");
const port = 3000
const cCPUs = require('os').cpus().length;
const cluster = require('cluster');
const caches = require('child_process')
let counter = 0
if (cluster.isPrimary) {
    // Create a worker for each CPU
    for (let i = 0; i < cCPUs; i++) {
        cluster.fork();
    }
    cluster.on('exit', function (worker) {
        console.log('worker ' + worker.process.pid + ' died.');
    });
} else {
    caches.spawn("memcached.exe", ['-p ' + (11211 + counter).toString()])
    let memJsClient = mem_js.Client.create('localhost:' + (11211 + counter++).toString())
    const app = new express.Server();
    app.get('/short/:id', async (req, res) => {
        const id = req.path_parameters.id;
        const url = await lib.findOrigin(id, memJsClient);
        if (url == null) {
            console.log("error")
            res.status(404)
            res.send()
        }
        else {
            res.send(url)
        }
    })
    app.post('/create', async (req, res) => {
        try {
            const url = req.query.url;
            const newID = await lib.shortUrl(url);
            res.send(newID);

        } catch (err) {
            res.send(err)
        }
    });
    app.listen(port).then (() => {})
}