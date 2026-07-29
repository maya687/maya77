import React, { useState, useEffect } from 'react';

// APIベースURLの設定 (Codespaces / AWS / ローカル環境対応)
const API_BASE = window.location.origin.includes('-3000.')
  ? window.location.origin.replace('-3000.', '-8080.') // Codespaces環境用
  : `${window.location.protocol}//${window.location.hostname}:8080`; // AWS / ローカル環境用

function App() {
  const [testUserId, setTestUserId] = useState(1);
  const [posts, setPosts] = useState([]);
  const [profile, setProfile] = useState(null);
  const [targetUserId, setTargetUserId] = useState(null);
  const [content, setContent] = useState('');
  const [imageFile, setImageFile] = useState(null);
  const [commentInputs, setCommentInputs] = useState({});
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editName, setEditName] = useState('');
  const [editBio, setEditBio] = useState('');
  const [editAvatar, setEditAvatar] = useState(null);

  // すべてのリクエストでCodespacesの認証クッキーを含めるための共通オプション
  const fetchOptions = (options = {}) => {
    return {
      ...options,
      credentials: 'include', // Codespacesのプロキシ認証を突破するために必須
    };
  };

  useEffect(() => {
    fetchProfile();
    fetchPosts();
  }, [testUserId, targetUserId]);

  const fetchProfile = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/users/profile`, fetchOptions({
        headers: { 'X-User-Id': String(testUserId) }
      }));
      if (res.ok) {
        const data = await res.json();
        setProfile(data);
        setEditName(data.username || '');
        setEditBio(data.bio || '');
      }
    } catch (e) {
      console.error("プロフィール取得エラー:", e);
    }
  };

  const fetchPosts = async () => {
    try {
      let url = `${API_BASE}/api/posts`;
      if (targetUserId) {
        url += `?user_id=${targetUserId}`;
      }
      const res = await fetch(url, fetchOptions({
        headers: { 'X-User-Id': String(testUserId) }
      }));
      if (res.ok) {
        const data = await res.json();
        setPosts(data || []);
      }
    } catch (e) {
      console.error("投稿取得エラー:", e);
    }
  };

  const handlePostSubmit = async (e) => {
    e.preventDefault();
    if (!content.trim() && !imageFile) return;

    const formData = new FormData();
    formData.append('content', content);
    if (imageFile) formData.append('image', imageFile);

    try {
      const res = await fetch(`${API_BASE}/api/posts`, fetchOptions({
        method: 'POST',
        headers: { 'X-User-Id': String(testUserId) },
        body: formData,
      }));
      if (res.ok) {
        setContent('');
        setImageFile(null);
        const fileInput = document.getElementById('image-input');
        if (fileInput) fileInput.value = '';
        fetchPosts();
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handlePostDelete = async (postId) => {
    if (!window.confirm('この投稿を削除しますか？')) return;
    try {
      const res = await fetch(`${API_BASE}/api/posts/delete?post_id=${postId}`,
        fetchOptions({
          method: 'POST',
          headers: { 'X-User-Id': String(testUserId) }
        }));
      if (res.ok) {
        fetchPosts();
      } else {
        alert('自分の投稿以外は削除できません。');
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleLike = async (postId) => {
    try {
      const res = await fetch(`${API_BASE}/api/likes`, fetchOptions({
        method: 'POST',
        headers: {
          'X-User-Id': String(testUserId),
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ post_id: postId }),
      }));
      if (res.ok) fetchPosts();
    } catch (e) {
      console.error(e);
    }
  };

  const handleCommentSubmit = async (e, postId) => {
    e.preventDefault();
    const commentText = commentInputs[postId];
    if (!commentText || !commentText.trim()) return;

    try {
      const res = await fetch(`${API_BASE}/api/comments`, fetchOptions({
        method: 'POST',
        headers: {
          'X-User-Id': String(testUserId),
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ post_id: postId, content: commentText }),
      }));
      if (res.ok) {
        setCommentInputs({ ...commentInputs, [postId]: '' });
        fetchPosts();
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleOpenModal = () => {
    if (profile) {
      setEditName(profile.username || '');
      setEditBio(profile.bio || '');
    }
    setEditAvatar(null);
    setIsModalOpen(true);
  };

  const handleProfileUpdate = async (e) => {
    e.preventDefault();
    const formData = new FormData();
    formData.append('username', editName);
    formData.append('bio', editBio);
    if (editAvatar) {
      formData.append('avatar', editAvatar);
    }

    try {
      const res = await fetch(`${API_BASE}/api/users/profile`, fetchOptions({
        method: 'POST',
        headers: {
          'X-User-Id': String(testUserId)
        },
        body: formData,
      }));
      if (res.ok) {
        setIsModalOpen(false);
        setEditAvatar(null);
        alert('プロフィールを更新しました！');
        await fetchProfile();
        await fetchPosts();
      } else {
        const errorText = await res.text();
        alert(`更新に失敗しました (サーバーエラー: ${errorText})`);
      }
    } catch (e) {
      console.error("通信エラー詳細:", e);
      alert('通信エラーが発生しました。バックエンドが起動しているか確認してください。');
    }
  };

  const renderImage = (url) => {
    if (!url) return null;
    return <img src={url.startsWith('http') ? url : `${API_BASE}${url}`} alt="Post asset" className="post-img" />;
  };

  const renderAvatar = (url) => {
    if (!url) return <div className="avatar-placeholder" />;
    return <img src={url.startsWith('http') ? url : `${API_BASE}${url}`} alt="Avatar" className="avatar-img" />;
  };

  return (
    <div className="app-container">
      <header className="navbar">
        <div className="nav-content">
          <h1 className="logo" onClick={() => setTargetUserId(null)}>Maya-X</h1>
          <div className="test-user-selector">
            <label htmlFor="user-id-select">テスト操作ユーザーID: </label>
            <input
              id="user-id-select"
              type="number"
              min="1"
              value={testUserId}
              onChange={(e) => setTestUserId(Number(e.target.value))}
              style={{ width: '50px', padding: '4px', marginLeft: '6px', textAlign: 'center' }}
            />
          </div>
          {profile && (
            <div className="nav-profile" onClick={handleOpenModal}>
              {renderAvatar(profile.avatar_url)}
              <span className="nav-username">{profile.username} (ID:{profile.id})</span>
            </div>
          )}
        </div>
      </header>

      <main className="main-layout">
        <section className="feed-section">
          {targetUserId && (
            <div className="filter-banner">
              <span>ユーザーID: {targetUserId} の投稿を表示中</span>
              <button onClick={() => setTargetUserId(null)}>すべての投稿に戻る</button>
            </div>
          )}

          {!targetUserId && (
            <form onSubmit={handlePostSubmit} className="post-form card">
              <textarea
                placeholder="今なにしてる？"
                value={content}
                onChange={(e) => setContent(e.target.value)}
              />
              <div className="form-actions">
                <input
                  id="image-input"
                  type="file"
                  accept="image/*"
                  onChange={(e) => setImageFile(e.target.files[0])}
                />
                <button type="submit" className="btn-primary">投稿する</button>
              </div>
            </form>
          )}

          <div className="posts-list">
            {posts.map((post) => (
              <article key={post.id} className="post-card card">
                <div className="post-header">
                  <div className="post-author" onClick={() => setTargetUserId(post.user_id)}>
                    {renderAvatar(post.avatar_url)}
                    <div>
                      <h3 className="author-name">{post.username}</h3>
                      <span className="post-time">{new Date(post.created_at).toLocaleString()}</span>
                    </div>
                  </div>
                  {post.user_id === testUserId && (
                    <button className="btn-delete" onClick={() => handlePostDelete(post.id)}>削除</button>
                  )}
                </div>

                <div className="post-body">
                  <p className="post-text">{post.content}</p>
                  {renderImage(post.image_url)}
                </div>

                <div className="post-actions">
                  <button
                    className={`btn-like ${post.liked_by_me ? 'liked' : ''}`}
                    onClick={() => handleLike(post.id)}
                  >
                    ❤️ {post.likes_count}
                  </button>
                </div>

                <div className="comments-section">
                  <div className="comments-list">
                    {(post.comments || []).map((comment) => (
                      <div key={comment.id} className="comment-item">
                        {renderAvatar(comment.avatar_url)}
                        <div className="comment-content">
                          <span className="comment-user">{comment.username}</span>
                          <p className="comment-text">{comment.content}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                  <form onSubmit={(e) => handleCommentSubmit(e, post.id)} className="comment-form">
                    <input
                      type="text"
                      placeholder="コメントを書く..."
                      value={commentInputs[post.id] || ''}
                      onChange={(e) => setCommentInputs({ ...commentInputs, [post.id]: e.target.value })}
                    />
                    <button type="submit">送信</button>
                  </form>
                </div>
              </article>
            ))}
            {posts.length === 0 && <p className="empty-message">投稿がありません。</p>}
          </div>
        </section>
      </main>

      {isModalOpen && profile && (
        <div className="modal-overlay">
          <div className="modal-card card">
            <h2>プロフィール編集 (ユーザーID: {profile.id})</h2>
            <form onSubmit={handleProfileUpdate}>
              <div className="form-group">
                <label>ユーザー名</label>
                <input
                  type="text"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                />
              </div>
              <div className="form-group">
                <label>自己紹介</label>
                <textarea
                  value={editBio}
                  onChange={(e) => setEditBio(e.target.value)}
                />
              </div>
              <div className="form-group">
                <label>アバター画像変更（任意）</label>
                <input
                  type="file"
                  accept="image/*"
                  onChange={(e) => setEditAvatar(e.target.files[0])}
                />
              </div>
              <div className="modal-actions">
                <button type="button" className="btn-secondary" onClick={() => setIsModalOpen(false)}>キャンセル</button>
                <button type="submit" className="btn-primary">保存する</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;