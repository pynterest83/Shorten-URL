import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, useParams } from 'react-router-dom';
import './App.css';
import { Trash2, Copy, Share } from 'lucide-react';

function URLShortener() {
  const [url, setUrl] = useState('');
  const [shortenedURLList, setShortenedURLList] = useState([]);

  useEffect(() => {
    const storedURLs = JSON.parse(localStorage.getItem('shortenedURLList'));
    console.log(storedURLs);
    if (!storedURLs) {
      return;
    }
    for (let i = 0; i < storedURLs.length; i++) {
      // Check if the stored URL is still valid
      const requestUrl = `http://localhost:8080/short/${storedURLs[i].shortId}`;
      fetch(requestUrl)
        .then((response) => {
          // response is alwasys 200 even if the URL is not found
          // if URL is not found, the response will be {"message":"URL not found"}
          // if response = {"message":"URL not found"}, then remove the URL from the list and remove it from local storage
          if (response.ok) {
            response.json().then((data) => {
              if (data.message === "URL not found") {
                storedURLs.splice(i, 1);
                setShortenedURLList(storedURLs);
                localStorage.setItem('shortenedURLList', JSON.stringify(storedURLs));
              }
            });
          }
        })
        .catch((error) => {
          console.error("Error checking URL:", error);
        });
    }
    if (storedURLs && Array.isArray(storedURLs)) {
      setShortenedURLList(storedURLs);
    }
  }, []);

  useEffect(() => {
    if (shortenedURLList.length > 0) {
      localStorage.setItem('shortenedURLList', JSON.stringify(shortenedURLList));
    }
  }, [shortenedURLList]);

  const handleShorten = async () => {
    if (!url) {
      alert("Please enter a URL to shorten.");
      return;
    }

    const requestUrl = `http://localhost:8080/create?url=${encodeURIComponent(url)}`;
    try {
      const response = await fetch(requestUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        const res = await response.text()
        const newID = JSON.parse(res).id;
        const shortened = `http://localhost:3000/short/${newID}`;
        const newShortenedURL = { fullUrl: url, shortUrl: shortened, shortId: newID };
        setShortenedURLList([...shortenedURLList, newShortenedURL]);
        setUrl('');
      } else {
        alert("Failed to shorten the URL. Please try again.");
      }
    } catch (error) {
      console.error("Error shortening the URL:", error);
      alert("An error occurred. Please try again.");
    }
  };

  const clearHistory = async () => {
    if (window.confirm('Are you sure you want to clear all URLs?')) {
      const ids = shortenedURLList.map((url) => url.shortId);
      try {
        const response = await fetch('http://localhost:8080/delete-urls', {
          method: 'DELETE',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(ids),
        });
        
        if (response.ok) {
          setShortenedURLList([]);
          localStorage.removeItem('shortenedURLList');
        } else {
          alert("Failed to delete URLs. Please try again.");
        }
      } catch (error) {
        console.error("Error deleting URLs:", error);
        alert("An error occurred. Please try again.");
      }
    }
  };
  
  const handleShare = (shortId) => {
    if (navigator.share) {
      navigator.share({
        title: 'Shortened URL',
        text: 'Check out this URL!',
        url: `http://localhost:3000/short/${shortId}`,
      });
    } else {
      alert('Share not supported on this browser.');
    }
  };

  const handleCopy = (shortId) => {
    navigator.clipboard.writeText(`http://localhost:3000/short/${shortId}`);
    alert(`Copied: http://localhost:3000/short/${shortId}`);
  };

  const handleDelete = async (index) => {
    const urlToDelete = shortenedURLList[index];
    try {
      const response = await fetch(`http://localhost:8080/delete-urls`, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify([urlToDelete.shortId]),
      });

      if (response.ok) {
        const updatedList = shortenedURLList.filter((_, i) => i !== index);
        setShortenedURLList(updatedList);

        if (updatedList.length === 0) {
          localStorage.removeItem('shortenedURLList'); // Clear storage if list is empty
        }
      } else {
        alert("Failed to delete URL. Please try again.");
      }
    } catch (error) {
      console.error("Error deleting URL:", error);
      alert("An error occurred. Please try again.");
    }
  };

  return (
    <div className="min-h-screen bg-gray-100 p-4 md:p-8">
      <div className="max-w-4xl mx-auto bg-white rounded-lg shadow-md p-6">
        <h2 className="text-2xl font-bold text-center mb-6">URLShrinker</h2>
        <div className="space-y-4">
          <input
            type="text"
            placeholder="Enter original link here"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            className="w-full p-2 border border-gray-300 rounded"
          />
          <button
            onClick={handleShorten}
            className="w-full bg-green-500 text-white p-2 rounded hover:bg-green-600 transition-colors"
          >
            Shorten URL
          </button>
        </div>

        {shortenedURLList.length > 0 && (
          <div className="mt-8">
            <h3 className="text-xl font-semibold mb-4">Your Shortened URLs:</h3>
            <div className="overflow-x-auto">
              <table className="w-full border">
                <thead>
                  <tr className="bg-black-200">
                    <th className="p-2 text-left">Full URL</th>
                    <th className="p-2 text-left">Short URL</th>
                    <th className="p-2 text-left">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {shortenedURLList.map((shortened, index) => (
                    <tr key={index} className="border-b">
                      <td className="p-2">
                        <a
                          href={shortened.fullUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-blue-500 hover:underline"
                        >
                          {shortened.fullUrl}
                        </a>
                      </td>
                      <td className="p-2">
                        <a
                          href={`http://localhost:3000/short/${shortened.shortId}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-blue-500 hover:underline"
                        >
                          {shortened.shortUrl}
                        </a>
                      </td>
                      <td className="p-2">
                        <div className="flex space-x-2">
                          <button
                            onClick={() => handleDelete(index)}
                            className="p-1 bg-red-500 text-white rounded hover:bg-red-600"
                            aria-label="Delete"
                          >
                            <Trash2 size={16} />
                          </button>
                          <button
                            onClick={() => handleCopy(shortened.shortId)}
                            className="p-1 bg-gray-500 text-white rounded hover:bg-gray-600"
                            aria-label="Copy"
                          >
                            <Copy size={16} />
                          </button>
                          <button
                            onClick={() => handleShare(shortened.shortId)}
                            className="p-1 bg-green-500 text-white rounded hover:bg-green-600"
                            aria-label="Share"
                          >
                            <Share size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <button
              onClick={clearHistory}
              className="mt-4 w-full bg-red-500 text-white p-2 rounded hover:bg-red-600 transition-colors"
            >
              Clear History
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function RedirectShort() {
  const { id } = useParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchOriginalUrl = async () => {
      try {
        const response = await fetch(`http://localhost:8080/short/${id}`);
        if (response.ok) {
          const data = await response.json();
          let url = data.originalUrl;
          if (!url.startsWith('http://') && !url.startsWith('https://')) {
            url = 'http://' + url;
          }
          window.location.replace(url);
        } else {
          setError('URL not found');
        }
      } catch (err) {
        console.error('Redirect error:', err);
        setError('Failed to fetch URL');
      } finally {
        setLoading(false);
      }
    };

    fetchOriginalUrl();
  }, [id]);

  if (loading) {
    return <div>Redirecting...</div>;
  }

  if (error) {
    return <div>Error: {error}</div>;
  }

  return null;
}

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<URLShortener />} />
        <Route path="/short/:id" element={<RedirectShort />} />
      </Routes>
    </Router>
  );
}

export default App;