import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, useParams } from 'react-router-dom';
import './App.css';

function URLShortener() {
  const [url, setUrl] = useState('');
  const [alias, setAlias] = useState('');
  const [shortenedURLList, setShortenedURLList] = useState([]);

  const handleShorten = async () => {
    if (!url) {
      alert("Please enter a URL to shorten.");
      return;
    }
  
    const requestUrl = `http://localhost:8080/create?url=${encodeURIComponent(url)}`; // Construct the URL
    console.log(requestUrl);
    try {
      const response = await fetch(requestUrl, {
        method: 'POST', // Use POST method
        headers: {
          'Content-Type': 'application/json',
        },
      });
  
      if (response.ok) {
        const newID = await response.text(); // Assuming the backend returns the new ID as plain text
        const shortened = `http://localhost:3000/short/${newID}`; // Use the newID received
        setShortenedURLList([...shortenedURLList, { fullUrl: url, shortUrl: shortened, shortId: newID}]); // Store newID directly
        setUrl('');
        setAlias('');
      } else {
        alert("Failed to shorten the URL. Please try again.");
      }
    } catch (error) {
      console.error("Error shortening the URL:", error);
      alert("An error occurred. Please try again.");
    }
  };

  const clearHistory = () => {
    if (window.confirm('Are you sure you want to clear all URLs?')) {
      setShortenedURLList([]);
    }
  };

  const handleShare = (shortId) => {
    if (navigator.share) {
      navigator.share({
        title: 'Shortened URL',
        text: 'Check out this URL!',
        url: `http://localhost:3000/short/${shortId}`, // Change to backend link
      });
    } else {
      alert('Share not supported on this browser.');
    }
  };

  const handleCopy = (shortId) => {
    navigator.clipboard.writeText(`http://localhost:3000/short/${shortId}`);
    alert(`Copied: http://localhost:3000/short/${shortId}`);
  };
  
  const handleDelete = (index) => {
    const updatedList = shortenedURLList.filter((_, i) => i !== index);
    setShortenedURLList(updatedList);
  };
  
  const handleEdit = (index) => {
    const editedUrl = prompt("Enter new URL:", shortenedURLList[index].fullUrl);
    if (editedUrl) {
      const updatedList = [...shortenedURLList];
      updatedList[index].fullUrl = editedUrl;
      setShortenedURLList(updatedList);
    }
  };
  
  return (
    <div className="app">
      <div className="url-shortener">
        <h2>URLShrinker</h2>
        <div className="input-container">
          <input
            type="text"
            placeholder="Enter original link here"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <div className="custom-link">
            <select>
              <option value="localhost:3000">localhost:3000</option>
            </select>
            <input
              type="text"
              placeholder="Enter alias"
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
            />
          </div>
          <button onClick={handleShorten}>Shorten URL</button>
        </div>

        {shortenedURLList.length > 0 && (
          <div className="shortened-list">
            <h3>Your Shortened URLs:</h3>
            <div className="table-container">
              <table>
                <thead>
                  <tr>
                    <th>Full URL</th>
                    <th style={{ width: '200px' }}>Short URL</th>
                    <th style={{ width: '150px' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {shortenedURLList.map((shortened, index) => (
                    <tr key={index}>
                      <td>
                        <a href={shortened.fullUrl} target="_blank" rel="noopener noreferrer">
                          {shortened.fullUrl}
                        </a>
                      </td>
                      <td>
                        <a href={`http://localhost:3000/short/${shortened.shortId}`} target="_blank" rel="noopener noreferrer">
                          {shortened.shortUrl}
                        </a>
                      </td>
                      <td style={{ display: 'flex', gap: '10px' }}>
                        <button className="action-button" onClick={() => handleEdit(index)}>Edit</button>
                        <button className="action-button" onClick={() => handleDelete(index)}>Delete</button>
                        <button className="action-button" onClick={() => handleCopy(shortened.shortId)}>Copy</button>
                        <button className="action-button" onClick={() => handleShare(shortened.shortId)}>Share</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <button className="clear-history" onClick={clearHistory}>Clear History</button>
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
          window.location.replace(data.originalUrl);
        } else {
          setError('URL not found');
        }
      } catch (err) {
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