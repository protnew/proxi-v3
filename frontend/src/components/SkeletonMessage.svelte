<div class="skeleton-msg">
  <div class="skeleton-avatar"></div>
  <div class="skeleton-content">
    <div class="skeleton-name"></div>
    <div class="skeleton-text"></div>
    <div class="skeleton-text short"></div>
  </div>
</div>

<style>
  .skeleton-msg {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;
    opacity: 0.7;
    animation: pulse 1.5s infinite ease-in-out;
  }
  
  .skeleton-avatar {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    background: var(--bg-glass);
    flex-shrink: 0;
  }
  
  .skeleton-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  
  .skeleton-name {
    width: 120px;
    height: 14px;
    background: var(--bg-glass);
    border-radius: 4px;
  }
  
  .skeleton-text {
    width: 100%;
    max-width: 400px;
    height: 16px;
    background: var(--bg-panel);
    border-radius: 8px;
  }
  
  .skeleton-text.short {
    width: 60%;
    max-width: 250px;
  }
  
  @keyframes pulse {
    0% { opacity: 0.4; }
    50% { opacity: 0.8; }
    100% { opacity: 0.4; }
  }
</style>
