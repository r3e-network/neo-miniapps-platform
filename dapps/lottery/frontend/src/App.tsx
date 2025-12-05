import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { WalletProvider } from './hooks/useWallet';
import { Header } from './components/Header';
import { HomePage } from './components/HomePage';
import { BuyTicketPage } from './components/BuyTicketPage';
import { MyTicketsPage } from './components/MyTicketsPage';
import { ResultsPage } from './components/ResultsPage';

function App() {
  return (
    <WalletProvider>
      <BrowserRouter>
        <div className="min-h-screen bg-gradient-to-br from-purple-900 via-blue-900 to-indigo-900">
          <Header />
          <main className="container mx-auto px-4 py-8">
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/buy" element={<BuyTicketPage />} />
              <Route path="/tickets" element={<MyTicketsPage />} />
              <Route path="/results" element={<ResultsPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </WalletProvider>
  );
}

export default App;
