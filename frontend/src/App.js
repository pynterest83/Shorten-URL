import React, { useState } from 'react';
import './App.css';

function App() {
  const [url, setUrl] = useState('');
  const [alias, setAlias] = useState('');
  const [shortenedURLList, setShortenedURLList] = useState([]);

  // Mock function to shorten URL
  const handleShorten = () => {
    // Generate alias if not provided
    const generatedAlias = alias || Math.random().toString(36).substring(7);
    const shortened = `tinyurl.com/${generatedAlias}`;

    // Add the new shortened URL to the list
    setShortenedURLList([...shortenedURLList, { fullUrl: url, shortUrl: shortened, clicks: 0 }]);

    // Reset fields
    setUrl('');
    setAlias('');
  };

  const clearHistory = () => {
    if (window.confirm('Are you sure you want to clear all URLs?')) {
      setShortenedURLList([]);
    }
  };

  const handleShare = (shortUrl) => {
    if (navigator.share) {
      navigator.share({
        title: 'Shortened URL',
        text: 'Check out this URL!',
        url: `https://${shortUrl}`,
      });
    } else {
      alert('Share not supported on this browser.');
    }
  };

  const handleCopy = (shortUrl) => {
    navigator.clipboard.writeText(`https://${shortUrl}`);
    alert(`Copied: https://${shortUrl}`);
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
          <option value="tinyurl.com">tinyurl.com</option>
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
                <th style={{ width: '30px' }}>Clicks</th>
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
                    <a href={`https://${shortened.shortUrl}`} target="_blank" rel="noopener noreferrer">
                      {shortened.shortUrl}
                    </a>
                  </td>
                  <td>{shortened.clicks}</td>
                  <td style={{ display: 'flex', gap: '10px' }}>
                    <button className="action-button" onClick={() => handleEdit(index)}></button>
                    <button className="action-button" onClick={() => handleDelete(index)}></button>
                    <button className="action-button" onClick={() => handleCopy(shortened.shortUrl)}></button>
                    <button className="action-button" onClick={() => handleShare(shortened.shortUrl)}></button>
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

export default App;